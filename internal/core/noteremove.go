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
}

// NewNoteRemover wires the usecase.
func NewNoteRemover(store mtt.KnowledgeStore, audit mtt.AuditStore, now func() time.Time) *NoteRemover {
	return &NoteRemover{store: store, audit: audit, now: now}
}

// Remove deletes slug. referents are the incoming backlink ids (from Backlinks).
func (r *NoteRemover) Remove(slug mtt.NoteSlug, referents []string, force bool, by, why string) error {
	if force {
		if missing := missingAttributionFields(true, true, by, why); len(missing) > 0 {
			return fmt.Errorf("%w: %s", ErrMissingAttribution, strings.Join(missing, ", "))
		}
	}
	if _, err := r.store.GetNote(slug); err != nil {
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
		return r.store.DeleteNote(slug)
	}
	entry := mtt.AuditEntry{At: r.now().UTC().Truncate(time.Second), Who: by, Why: why, Action: "note rm --force", TaskID: mtt.TaskID(slug)}
	if err := r.audit.Append(entry); err != nil {
		return fmt.Errorf("audit append for %q: %w", slug, err)
	}
	return r.store.DeleteNote(slug)
}
