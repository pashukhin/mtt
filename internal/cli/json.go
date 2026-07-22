package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/pashukhin/mtt/internal/core"
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
// current status (the status's description + the onward moves) and the task's
// transition history. It anonymously embeds taskJSON so those fields stay
// top-level and the shared list/edit/status `--json` (which use taskJSON
// directly) are untouched. All added fields are omitempty, so a status with no
// description, no onward moves, and no history adds nothing.
type showJSON struct {
	taskJSON
	StatusDescription string         `json:"status_description,omitempty"`
	Next              []nextMoveJSON `json:"next,omitempty"`
	History           []historyJSON  `json:"history,omitempty"`
	Refs              []refJSON      `json:"refs,omitempty"`
	Backlinks         []backlinkJSON `json:"backlinks,omitempty"`
}

// nextMoveJSON is one onward transition from the current status (the target and
// its transition description, if any).
type nextMoveJSON struct {
	Name        string `json:"name,omitempty"`
	To          string `json:"to"`
	Description string `json:"description,omitempty"`
}

// historyJSON is one transition audit entry in `mtt show --json` (session
// 008.97/U3): the human view renders history, so the JSON consumer needs it too
// (checks + attribution).
type historyJSON struct {
	At     string      `json:"at"`
	By     string      `json:"by,omitempty"`
	Role   string      `json:"role,omitempty"`
	Why    string      `json:"why,omitempty"`
	From   string      `json:"from"`
	To     string      `json:"to"`
	Checks []checkJSON `json:"checks,omitempty"`
}

// checkJSON is one gate command result (the command and its exit code).
type checkJSON struct {
	Cmd  string `json:"cmd"`
	Exit int    `json:"exit"`
}

// toShowJSON builds the show view from a task, its current status's description,
// and the onward transitions. Next/History stay nil (omitted) when empty.
func toShowJSON(t mtt.Task, statusDesc string, onward []mtt.Transition) showJSON {
	sj := showJSON{taskJSON: toTaskJSON(t), StatusDescription: statusDesc}
	for _, e := range onward {
		sj.Next = append(sj.Next, nextMoveJSON{
			Name: e.Name, To: string(e.To),
			Description: core.ExpandText(e.Description, string(t.ID), string(t.Type), string(e.From), string(e.To)),
		})
	}
	for _, h := range t.History {
		hj := historyJSON{
			At: h.At.UTC().Format(time.RFC3339), By: h.By, Role: h.Role, Why: h.Why,
			From: string(h.From), To: string(h.To),
		}
		for _, c := range h.Checks {
			hj.Checks = append(hj.Checks, checkJSON{Cmd: c.Cmd, Exit: c.Exit})
		}
		sj.History = append(sj.History, hj)
	}
	return sj
}

// versionJSON is `mtt version --json`.
type versionJSON struct {
	Version string `json:"version"`
}

// initJSON is `mtt init --json`: the created-config summary (absolute path).
type initJSON struct {
	Path     string `json:"path"`
	Template string `json:"template"`
	Name     string `json:"name"`
	Created  bool   `json:"created"`
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
