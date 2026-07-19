package cli

import (
	"fmt"

	"github.com/pashukhin/mtt/pkg/mtt"
)

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
