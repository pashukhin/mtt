package cli

import (
	"encoding/json"
	"strings"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// taskEnvBuilder returns the injected env-builder that core (Transitioner /
// EventEmitter) calls per move/event: given a task it returns the MTT_TASK_*
// environment a gate/post/event command sees. The closure captures the store so
// it can compute children-with-statuses; serialization lives HERE, so core stays
// serialization-free (pkg/mtt has no json tags). Best-effort: a List error just
// omits children; NUL is stripped from values (os/exec refuses a NUL in an env
// value — the exec adapter strips too, this is defense in depth).
//
// The {{...}} template surface (core's cmdContext) is unchanged and stays
// shape-safe; free text (title/tags) rides the environment as DATA, never
// interpolated into the command string.
func taskEnvBuilder(store mtt.TaskStore) func(mtt.Task) map[string]string {
	return func(t mtt.Task) map[string]string {
		env := map[string]string{
			"MTT_TASK_ID":       string(t.ID),
			"MTT_TASK_TYPE":     string(t.Type),
			"MTT_TASK_STATUS":   string(t.Status),
			"MTT_TASK_TITLE":    t.Title,
			"MTT_TASK_PARENT":   string(t.Parent),
			"MTT_TASK_PRIORITY": string(t.Priority),
			"MTT_TASK_TAGS":     strings.Join(t.Tags, ","),
		}
		if b, err := json.Marshal(toTaskJSON(t)); err == nil {
			env["MTT_TASK_JSON"] = string(b)
		}
		if all, err := store.List(); err == nil {
			type child struct {
				ID     string `json:"id"`
				Type   string `json:"type"`
				Status string `json:"status"`
			}
			kids := []child{} // non-nil so it marshals to "[]", never "null"
			for _, c := range all {
				if c.Parent == t.ID {
					kids = append(kids, child{ID: string(c.ID), Type: string(c.Type), Status: string(c.Status)})
				}
			}
			if b, err := json.Marshal(kids); err == nil {
				env["MTT_TASK_CHILDREN_JSON"] = string(b)
			}
		}
		for k, v := range env {
			env[k] = strings.ReplaceAll(v, "\x00", "") // exec refuses NUL in env values
		}
		return env
	}
}
