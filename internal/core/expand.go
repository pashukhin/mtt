package core

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// cmdContext is the whitelist of placeholder fields exposed to a command
// template. Only these shape-safe identifiers are available — never free text
// (title/description) — so expansion cannot inject shell metacharacters, and a
// stray {{.Title}} is a template error (the struct's shape self-enforces the
// whitelist).
type cmdContext struct {
	ID   string
	Type string
	From string
	To   string
}

// taskEventContext is the placeholder whitelist for task lifecycle events:
// {{.ID}}, {{.Type}}, {{.Event}}. No From/To — an event is not an edge; a
// stray {{.From}} is a template error (the struct shape self-enforces it).
// Type carries the CONFIG's type name (membership-checked by the emitter),
// never the raw on-disk value (the c15-class guard, spec §4).
type taskEventContext struct {
	ID    string
	Type  string
	Event string
}

// noteEventContext is the placeholder whitelist for note lifecycle events:
// {{.Slug}} (structurally validated kebab ASCII) and {{.Event}}.
type noteEventContext struct {
	Slug  string
	Event string
}

// expandCommands renders each command's Run (and, recursively, its Rollback.Run)
// against ctx, returning new commands with expanded strings and unchanged
// timeouts. Expansion is eager and up-front (before the gate), so a malformed
// template in a command OR its rollback aborts the transition before any side
// effect runs. A malformed template (Parse) or a reference to an unexposed field
// (Execute) is an error.
func expandCommands(cmds []mtt.Command, data any) ([]mtt.Command, error) {
	if len(cmds) == 0 {
		return nil, nil
	}
	out := make([]mtt.Command, 0, len(cmds))
	for _, c := range cmds {
		ec, err := expandOne(c, data)
		if err != nil {
			return nil, err
		}
		out = append(out, ec)
	}
	return out, nil
}

// expandOne expands a command's Run and (recursively) its Rollback against ctx.
// A compensator is a leaf (Config.Validate guarantees rollback.Rollback == nil),
// so the recursion is at most one level deep.
func expandOne(c mtt.Command, data any) (mtt.Command, error) {
	run, err := expandTemplate(c.Run, data)
	if err != nil {
		return mtt.Command{}, err
	}
	out := mtt.Command{Run: run, Timeout: c.Timeout}
	if c.Rollback != nil {
		rb, err := expandOne(*c.Rollback, data)
		if err != nil {
			return mtt.Command{}, err
		}
		out.Rollback = &rb
	}
	return out, nil
}

// ExpandText expands {{.ID}}/{{.Type}}/{{.From}}/{{.To}} in raw for SHOWN guidance
// (descriptions), returning the raw text UNCHANGED on any parse/execute error —
// guidance is informational and must never break a command. (Gate commands use the
// strict expandCommands, which aborts on error; this is the lenient sibling.) Note
// the fallback is whole-string: a template mixing a valid {{.ID}} with an unknown
// field renders entirely raw, not partially.
func ExpandText(raw, id, typ, from, to string) string {
	out, err := expandTemplate(raw, cmdContext{ID: id, Type: typ, From: from, To: to})
	if err != nil {
		return raw
	}
	return out
}

// expandTemplate renders one raw template string against data (one of the
// whitelist context structs — never free text).
func expandTemplate(raw string, data any) (string, error) {
	tmpl, err := template.New("cmd").Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse command %q: %w", raw, err)
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, data); err != nil {
		return "", fmt.Errorf("expand command %q: %w", raw, err)
	}
	return b.String(), nil
}
