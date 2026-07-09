package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// taskJSON is the CLI's machine-readable view of a task. JSON is a presentation
// concern, so the tags live here, not on the pure domain type (mirrors the YAML
// adapter's DTO). Reserved collections are omitted until later phases populate
// them; adding fields later is additive.
type taskJSON struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Title       string   `json:"title,omitempty"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority,omitempty"`
	Parent      string   `json:"parent,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Created     string   `json:"created"`
	Updated     string   `json:"updated"`
	Description string   `json:"description,omitempty"`
}

// toTaskJSON maps a domain task to its JSON view (RFC3339 UTC timestamps).
func toTaskJSON(t mtt.Task) taskJSON {
	return taskJSON{
		ID: string(t.ID), Type: string(t.Type), Title: t.Title, Status: string(t.Status), Priority: string(t.Priority), Parent: string(t.Parent),
		Tags:        t.Tags,
		Created:     t.Created.UTC().Format(time.RFC3339),
		Updated:     t.Updated.UTC().Format(time.RFC3339),
		Description: t.Description,
	}
}

// showJSON is `mtt show --json`: the task view plus the flow guidance for its
// current status (the status's description + the onward moves). It anonymously
// embeds taskJSON so those fields stay top-level and the shared list/edit/status
// `--json` (which use taskJSON directly) are untouched. Both guidance fields are
// omitempty, so a status with no description and no onward moves adds nothing.
type showJSON struct {
	taskJSON
	StatusDescription string         `json:"status_description,omitempty"`
	Next              []nextMoveJSON `json:"next,omitempty"`
}

// nextMoveJSON is one onward transition from the current status (the target and
// its transition description, if any).
type nextMoveJSON struct {
	To          string `json:"to"`
	Description string `json:"description,omitempty"`
}

// toShowJSON builds the show view from a task, its current status's description,
// and the onward transitions. Next stays nil (omitted) when there are none.
func toShowJSON(t mtt.Task, statusDesc string, onward []mtt.Transition) showJSON {
	sj := showJSON{taskJSON: toTaskJSON(t), StatusDescription: statusDesc}
	for _, e := range onward {
		sj.Next = append(sj.Next, nextMoveJSON{To: string(e.To), Description: e.Description})
	}
	return sj
}

// writeJSON marshals v as indented JSON with a trailing newline (stable diff).
func writeJSON(w io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}
