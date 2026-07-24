// Package core holds mtt's usecase logic. It depends only on the pkg/mtt domain
// contract and its ports — never on a concrete adapter.
package core

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// validateTitle enforces a single-line title: a title renders as one row in
// list/tree, so an embedded newline would split one task into rows
// indistinguishable from separate tasks for line-oriented consumers. Shared by
// Add and Edit; the description stays free-form (multi-line allowed).
func validateTitle(title string) error {
	if strings.ContainsAny(title, "\n\r") {
		return fmt.Errorf("title must be a single line (no newlines); use the description for multi-line text")
	}
	return nil
}

// Adder is the create-a-task usecase. It resolves the type, enforces placement,
// picks the entry status, stamps timestamps, and persists via the TaskStore.
type Adder struct {
	store mtt.TaskStore
	cfg   mtt.Config
	now   func() time.Time
	ev    *EventEmitter
}

// NewAdder wires the usecase. now is injected so timestamps are deterministic in
// tests (pass time.Now in production); ev fires the create event (nil = none).
func NewAdder(store mtt.TaskStore, cfg mtt.Config, now func() time.Time, ev *EventEmitter) *Adder {
	return &Adder{store: store, cfg: cfg, now: now, ev: ev}
}

// AddParams are the inputs to Add. TypeName empty selects the default type.
// NoParent creates a parent-requiring type at top level (a conscious exception).
type AddParams struct {
	Title       string
	TypeName    mtt.TypeName
	Parent      mtt.TaskID
	NoParent    bool
	Description string
	Priority    mtt.Priority // unset by default (not medium)
	DependsOn   []mtt.TaskID // blocking edges set at creation (targets validated)
	Tags        []string     // explicit tags; unioned with #hashtags from title/description
	Refs        []mtt.Ref    // informational references set at creation (canonicalized; not verified here)
	Events      EventOptions // lifecycle-event bypass + attribution (t66)
}

// Add creates one task and returns it with the adapter-minted ID. A
// *PostActionError return carries the PERSISTED task (the create happened;
// only the lifecycle event's finalization failed — exit 5, like Transitioner).
func (a *Adder) Add(p AddParams) (mtt.Task, error) {
	if err := p.Events.Preflight(); err != nil {
		return mtt.Task{}, err
	}
	if p.Title == "" && p.Description == "" {
		return mtt.Task{}, fmt.Errorf("provide a title or a description")
	}
	if err := validateTitle(p.Title); err != nil {
		return mtt.Task{}, err
	}
	var (
		typ mtt.Type
		ok  bool
	)
	if p.TypeName != "" {
		if typ, ok = a.cfg.TypeByName(p.TypeName); !ok {
			return mtt.Task{}, fmt.Errorf("unknown type %q", p.TypeName)
		}
	} else if typ, ok = a.cfg.DefaultType(); !ok {
		return mtt.Task{}, fmt.Errorf("no types configured")
	}
	var parent mtt.TaskID
	switch {
	case p.Parent != "":
		pt, err := a.store.Get(p.Parent)
		if err != nil {
			if errors.Is(err, mtt.ErrNotFound) {
				return mtt.Task{}, fmt.Errorf("parent %q: %w", p.Parent, mtt.ErrNotFound)
			}
			return mtt.Task{}, fmt.Errorf("load parent %q: %w", p.Parent, err)
		}
		if !typ.AcceptsParent(pt.Type) {
			return mtt.Task{}, fmt.Errorf("type %q cannot be placed under type %q (allowed parents: %v)", typ.Name, pt.Type, typ.Parents)
		}
		parent = pt.ID
	case !typ.IsRoot() && !p.NoParent:
		return mtt.Task{}, fmt.Errorf("type %q requires a parent; use --parent <id> (or --no-parent to create it at the top level)", typ.Name)
	}
	deps, err := a.resolveDeps(p.DependsOn)
	if err != nil {
		return mtt.Task{}, err
	}
	initial, ok := typ.InitialStatus()
	if !ok {
		return mtt.Task{}, fmt.Errorf("type %q has no initial status", typ.Name)
	}
	now := a.now().UTC().Truncate(time.Second)
	tags := canonicalTags(p.Tags, mtt.ExtractTags(p.Title), mtt.ExtractTags(p.Description))
	var refs []mtt.Ref
	if len(p.Refs) > 0 {
		refs = canonicalRefs(p.Refs)
	}
	created, err := a.store.Create(mtt.Task{
		Type:        typ.Name,
		Title:       p.Title,
		Status:      initial.Name,
		Priority:    p.Priority,
		Parent:      parent,
		Tags:        tags,
		DependsOn:   deps,
		Refs:        refs,
		Description: p.Description,
		Created:     now,
		Updated:     now,
	})
	if err != nil {
		return mtt.Task{}, err
	}
	if err := a.ev.TaskEvent(mtt.EventCreate, created, "add", p.Events); err != nil {
		return created, err // finalization failure: the task IS persisted (exit 5)
	}
	return created, nil
}

// resolveDeps validates that each depends-on target exists (via TaskStore.Get)
// and returns the deduped list. A missing target wraps mtt.ErrNotFound. No
// cycle check is needed: the new task's id is unminted, so it cannot be a target.
func (a *Adder) resolveDeps(ids []mtt.TaskID) ([]mtt.TaskID, error) {
	seen := map[mtt.TaskID]bool{}
	var out []mtt.TaskID
	for _, dep := range ids {
		if seen[dep] {
			continue
		}
		seen[dep] = true
		if _, err := a.store.Get(dep); err != nil {
			if errors.Is(err, mtt.ErrNotFound) {
				return nil, fmt.Errorf("depends-on target %q: %w", dep, mtt.ErrNotFound)
			}
			return nil, fmt.Errorf("load depends-on target %q: %w", dep, err)
		}
		out = append(out, dep)
	}
	return out, nil
}
