package yaml

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	roottemplates "github.com/pashukhin/mtt/templates"
)

// builtin returns the embedded bytes for a built-in template name, or false.
func builtin(name string) ([]byte, bool) {
	if name == "default" {
		return roottemplates.Default(), true
	}
	return nil, false
}

// templateNames returns the valid built-in template names, for the
// unknown-template error and discoverability.
func templateNames() []string { return roottemplates.BuiltinNames() }

// renderTemplate renders the named BUILT-IN template, substituting the project
// name. External templates never reach here (they are written verbatim).
func renderTemplate(name, projectName string) ([]byte, error) {
	raw, ok := builtin(name)
	if !ok {
		return nil, fmt.Errorf("unknown template %q (valid: %s)", name, strings.Join(templateNames(), ", "))
	}
	tmpl, err := template.New(name).Option("missingkey=error").Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("parse template %q: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct{ Name string }{Name: projectName}); err != nil {
		return nil, fmt.Errorf("render template %q: %w", name, err)
	}
	return buf.Bytes(), nil
}
