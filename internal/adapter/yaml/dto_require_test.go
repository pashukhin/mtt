package yaml

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_DecodesPerEdgeRequire(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".mtt")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `version: 1
project: {name: demo}
types:
  - name: task
    prefix: t
    default: true
    statuses:
      - {name: a, kind: initial, default: true}
      - {name: b, kind: terminal}
    transitions:
      - from: a
        to: b
        name: go
        require: {who: true, why: true}
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	edge := cfg.Types[0].Transitions[0]
	if !edge.Require.Who || !edge.Require.Why {
		t.Fatalf("per-edge require not decoded: %+v", edge.Require)
	}
}
