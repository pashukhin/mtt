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

// expandCommands renders each command's Run (and, recursively, its Rollback.Run)
// against ctx, returning new commands with expanded strings and unchanged
// timeouts. Expansion is eager and up-front (before the gate), so a malformed
// template in a command OR its rollback aborts the transition before any side
// effect runs. A malformed template (Parse) or a reference to an unexposed field
// (Execute) is an error.
func expandCommands(cmds []mtt.Command, ctx cmdContext) ([]mtt.Command, error) {
	if len(cmds) == 0 {
		return nil, nil
	}
	out := make([]mtt.Command, 0, len(cmds))
	for _, c := range cmds {
		ec, err := expandOne(c, ctx)
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
func expandOne(c mtt.Command, ctx cmdContext) (mtt.Command, error) {
	run, err := expandTemplate(c.Run, ctx)
	if err != nil {
		return mtt.Command{}, err
	}
	out := mtt.Command{Run: run, Timeout: c.Timeout}
	if c.Rollback != nil {
		rb, err := expandOne(*c.Rollback, ctx)
		if err != nil {
			return mtt.Command{}, err
		}
		out.Rollback = &rb
	}
	return out, nil
}

// expandTemplate renders one raw template string against ctx.
func expandTemplate(raw string, ctx cmdContext) (string, error) {
	tmpl, err := template.New("cmd").Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse command %q: %w", raw, err)
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, ctx); err != nil {
		return "", fmt.Errorf("expand command %q: %w", raw, err)
	}
	return b.String(), nil
}
