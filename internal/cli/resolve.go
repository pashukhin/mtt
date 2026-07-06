package cli

import (
	"errors"
	"fmt"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// argOrEmpty returns the first arg or "" — the seam between an explicit id and
// the current-task fallback.
func argOrEmpty(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

// resolveTaskID resolves the task a single-task verb acts on: an explicit id
// wins; otherwise the current-task pointer (validated at the point of use, so a
// stale pointer yields an actionable error); otherwise an error. Never consults
// current for list/tree/dep/ready/bulk — those pass an explicit target or operate
// on a set.
func resolveTaskID(root, explicit string) (mtt.TaskID, error) {
	if explicit != "" {
		return mtt.NewTaskID(explicit) // existence is the caller's own Get
	}
	id, ok, err := yaml.NewCurrent(root).Current()
	if err != nil {
		return "", err
	}
	if !ok {
		return "", errors.New("no task id given and no current task set (run `mtt use <id>`)")
	}
	if _, err := yaml.NewTaskStore(root).Get(id); err != nil {
		if errors.Is(err, mtt.ErrNotFound) {
			return "", fmt.Errorf("current task %q no longer exists; run `mtt use <id>` or `mtt use --clear`", id)
		}
		return "", err
	}
	return id, nil
}

// statusInAnyFlow reports whether name is a status in any configured type's flow
// (used to give `mtt <status>` with no current a helpful error vs unknown-command).
func statusInAnyFlow(cfg mtt.Config, name string) bool {
	sn := mtt.StatusName(name)
	for _, t := range cfg.Types {
		if _, ok := t.StatusKind(sn); ok {
			return true
		}
	}
	return false
}
