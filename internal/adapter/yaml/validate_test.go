package yaml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goodTemplate = `version: 1
project: {name: x}
types:
  - name: task
    prefix: t
    default: true
    statuses:
      - {name: tbd, kind: initial}
      - {name: doing, kind: active}
      - {name: done, kind: terminal}
    transitions:
      - {from: tbd, to: doing}
      - {from: doing, to: done}
`

// mustWriteConfig writes content to <dir>/.mtt/config.yaml (creating .mtt).
func mustWriteConfig(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".mtt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".mtt", "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestValidateTemplateBytes(t *testing.T) {
	if err := ValidateTemplateBytes([]byte(goodTemplate)); err != nil {
		t.Fatalf("good template: %v", err)
	}
	// bad prefix (shell metachar) — the injection case checkPrefixes must reject
	bad := strings.Replace(goodTemplate, "prefix: t", "prefix: \"t;rm\"", 1)
	if err := ValidateTemplateBytes([]byte(bad)); err == nil {
		t.Fatal("shell-metachar prefix must be rejected")
	}
	// domain-invalid but provider-valid: a transition to a non-existent status
	dom := strings.Replace(goodTemplate, "to: done", "to: ghost", 1)
	if err := ValidateTemplateBytes([]byte(dom)); err == nil {
		t.Fatal("domain-invalid template must be rejected by Config.Validate")
	}
	// not YAML at all
	if err := ValidateTemplateBytes([]byte("::not yaml::")); err == nil {
		t.Fatal("unparseable template must error")
	}
}

func TestLoadDoesNotRunConfigValidate(t *testing.T) {
	// A provider-valid but DOMAIN-invalid config still LOADS (Load's contract:
	// domain invariants are the caller's). ValidateTemplateBytes rejects it, but
	// Load must not — else read commands regress.
	dir := t.TempDir()
	mustWriteConfig(t, dir, strings.Replace(goodTemplate, "to: done", "to: ghost", 1))
	if _, _, err := Load(dir); err != nil {
		t.Fatalf("Load must not run Config.Validate (domain-invalid still loads): %v", err)
	}
}
