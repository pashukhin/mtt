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
	ID          string `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title,omitempty"`
	Status      string `json:"status"`
	Priority    string `json:"priority,omitempty"`
	Parent      string `json:"parent,omitempty"`
	Created     string `json:"created"`
	Updated     string `json:"updated"`
	Description string `json:"description,omitempty"`
}

// toTaskJSON maps a domain task to its JSON view (RFC3339 UTC timestamps).
func toTaskJSON(t mtt.Task) taskJSON {
	return taskJSON{
		ID: string(t.ID), Type: string(t.Type), Title: t.Title, Status: string(t.Status), Priority: string(t.Priority), Parent: string(t.Parent),
		Created:     t.Created.UTC().Format(time.RFC3339),
		Updated:     t.Updated.UTC().Format(time.RFC3339),
		Description: t.Description,
	}
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
