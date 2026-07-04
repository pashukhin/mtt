// Package core holds mtt's usecase logic. It depends only on the pkg/mtt domain
// contract and its ports — never on a concrete adapter.
package core

import (
	"fmt"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// Adder is the create-a-task usecase. It resolves the type, enforces placement,
// picks the entry status, stamps timestamps, and persists via the TaskStore.
type Adder struct {
	store mtt.TaskStore
	cfg   mtt.Config
	now   func() time.Time
}

// NewAdder wires the usecase. now is injected so timestamps are deterministic in
// tests (pass time.Now in production).
func NewAdder(store mtt.TaskStore, cfg mtt.Config, now func() time.Time) *Adder {
	return &Adder{store: store, cfg: cfg, now: now}
}

// AddParams are the inputs to Add. TypeName empty selects the default type.
// NoParent creates a parent-requiring type at top level (a conscious exception).
type AddParams struct {
	Title       string
	TypeName    string
	NoParent    bool
	Description string
}

// Add creates one task and returns it with the adapter-minted ID.
func (a *Adder) Add(p AddParams) (mtt.Task, error) {
	if p.Title == "" && p.Description == "" {
		return mtt.Task{}, fmt.Errorf("provide a title or a description")
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
	if !typ.IsRoot() && !p.NoParent {
		return mtt.Task{}, fmt.Errorf("type %q requires a parent; use --parent (session 004) or --no-parent to create it top-level", typ.Name)
	}
	initial, ok := typ.InitialStatus()
	if !ok {
		return mtt.Task{}, fmt.Errorf("type %q has no initial status", typ.Name)
	}
	now := a.now().UTC().Truncate(time.Second)
	return a.store.Create(mtt.Task{
		Type:        typ.Name,
		Title:       p.Title,
		Status:      initial.Name,
		Description: p.Description,
		Created:     now,
		Updated:     now,
	})
}
