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

// expandCommands renders each command's Run template against ctx, returning new
// commands with the expanded Run and the unchanged Timeout. A malformed template
// (Parse) or a reference to an unexposed field (Execute) is an error.
func expandCommands(cmds []mtt.Command, ctx cmdContext) ([]mtt.Command, error) {
	if len(cmds) == 0 {
		return nil, nil
	}
	out := make([]mtt.Command, 0, len(cmds))
	for _, c := range cmds {
		tmpl, err := template.New("cmd").Parse(c.Run)
		if err != nil {
			return nil, fmt.Errorf("parse command %q: %w", c.Run, err)
		}
		var b strings.Builder
		if err := tmpl.Execute(&b, ctx); err != nil {
			return nil, fmt.Errorf("expand command %q: %w", c.Run, err)
		}
		out = append(out, mtt.Command{Run: b.String(), Timeout: c.Timeout})
	}
	return out, nil
}
