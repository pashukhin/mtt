package yaml

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed templates/default.yaml templates/coding.yaml templates/hierarchy.yaml
var templatesFS embed.FS

// templateFiles maps init template names to their embedded paths.
var templateFiles = map[string]string{
	"default":   "templates/default.yaml",
	"coding":    "templates/coding.yaml",
	"hierarchy": "templates/hierarchy.yaml",
}

// renderTemplate renders the named init template, substituting the project name.
func renderTemplate(name, projectName string) ([]byte, error) {
	path, ok := templateFiles[name]
	if !ok {
		return nil, fmt.Errorf("unknown template %q", name)
	}
	raw, err := templatesFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read template %q: %w", name, err)
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
