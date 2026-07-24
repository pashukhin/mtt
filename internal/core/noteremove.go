package core

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// NoteRemover deletes a note, refusing by default when other carriers reference it
// (referents supplied by the caller — core stays store-agnostic, no TaskStore). It
// mirrors Remover's dangerous-ops policy: --force forces who+why and audits before
// deleting.
type NoteRemover struct {
	store mtt.KnowledgeStore
	audit mtt.AuditStore
	now   func() time.Time
	ev    *EventEmitter
}

// NewNoteRemover wires the usecase; ev fires the note delete event (nil = none).
func NewNoteRemover(store mtt.KnowledgeStore, audit mtt.AuditStore, now func() time.Time, ev *EventEmitter) *NoteRemover {
	return &NoteRemover{store: store, audit: audit, now: now, ev: ev}
}

// Remove deletes slug. referents are the incoming backlink ids (from Backlinks).
// Mirrors Remover exactly: under force+noRun ONE audit record signs both
// (action "note rm --force --no-run", pre-delete ordering — pin iii) and no
// pipeline runs; under a plain bypass the emitter writes the skip record.
func (r *NoteRemover) Remove(slug mtt.NoteSlug, referents []string, force bool, by, why string, noRun bool) error {
	if force {
		if missing := missingAttributionFields(true, true, by, why); len(missing) > 0 {
			return fmt.Errorf("%w: %s", ErrMissingAttribution, strings.Join(missing, ", "))
		}
	}
	if noRun && !force { // the force branch above already demanded who+why
		if err := (EventOptions{NoRun: true, By: by, Why: why}).Preflight(); err != nil {
			return err
		}
	}
	note, err := r.store.GetNote(slug)
	if err != nil {
		if errors.Is(err, mtt.ErrNotFound) {
			return fmt.Errorf("note %q: %w", slug, mtt.ErrNotFound)
		}
		return fmt.Errorf("load note %q: %w", slug, err)
	}
	if !force {
		if len(referents) > 0 {
			return fmt.Errorf("note %q is referenced by %s; use --force to delete anyway",
				slug, strings.Join(referents, ", "))
		}
		if err := r.store.DeleteNote(slug); err != nil {
			return err
		}
		return r.ev.NoteEvent(mtt.EventDelete, note, "note rm", EventOptions{NoRun: noRun, By: by, Why: why})
	}
	action := "note rm --force"
	if noRun {
		action = "note rm --force --no-run"
	}
	entry := mtt.AuditEntry{At: r.now().UTC().Truncate(time.Second), Who: by, Why: why, Action: action, TaskID: mtt.TaskID(slug)}
	if err := r.audit.Append(entry); err != nil {
		return fmt.Errorf("audit append for %q: %w", slug, err)
	}
	if err := r.store.DeleteNote(slug); err != nil {
		return err
	}
	if noRun {
		return nil // the force record above already signed the bypass (pin iii)
	}
	return r.ev.NoteEvent(mtt.EventDelete, note, "note rm", EventOptions{})
}
