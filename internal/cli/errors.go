package cli

import (
	"errors"
	"fmt"

	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// isNotFound reports whether err wraps mtt.ErrNotFound (the not-found taxonomy).
func isNotFound(err error) bool { return errors.Is(err, mtt.ErrNotFound) }

// attributionHint tells the user how to supply who/why after an exit-2 refusal.
const attributionHint = "" +
	"  set 'who': add `author: <name>` to .mtt/config.local.yaml, or `export MTT_BY=<name>`, or pass `--who <name>`\n" +
	"  set 'why': pass `--why \"<reason>\"`\n"

// notFoundHint points at the discovery commands after an exit-4 miss.
const notFoundHint = "" +
	"  check the id — 'mtt roadmap' or 'mtt list' show existing task ids ('mtt note list' for notes)\n"

// exitHint returns a trailing, actionable hint block for the context-free error
// sentinels, or "" when the error carries its own context (post-action, invalid
// transition) or is unrelated. Printed by Execute under the `error:` line.
func exitHint(err error) string {
	switch {
	case errors.Is(err, core.ErrMissingAttribution):
		return attributionHint
	case errors.Is(err, mtt.ErrNotFound):
		return notFoundHint
	default:
		return ""
	}
}

// taskNotFound is the uniform "task not found" error for single-task-by-id
// commands. Wrapping mtt.ErrNotFound lets exitCode map every such path to 4.
func taskNotFound(id mtt.TaskID) error {
	return fmt.Errorf("task %q: %w", id, mtt.ErrNotFound)
}

// noteNotFound is the uniform "note not found" error for single-note-by-slug
// commands. Wrapping mtt.ErrNotFound lets exitCode map it to 4 (like taskNotFound).
func noteNotFound(slug mtt.NoteSlug) error {
	return fmt.Errorf("note %q: %w", string(slug), mtt.ErrNotFound)
}
