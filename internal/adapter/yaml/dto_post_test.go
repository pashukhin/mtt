package yaml

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_DecodesPostCommands(t *testing.T) {
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
        post:
          - 'git add .mtt && git commit -m "{{.ID}}: {{.From}} → {{.To}}" -- .mtt'
          - {run: echo done, timeout: 30s}
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	post := cfg.Types[0].Transitions[0].Post
	if len(post) != 2 || post[0].Run == "" || post[1].Run != "echo done" || post[1].Timeout == 0 {
		t.Fatalf("post not decoded: %+v", post)
	}
}

func TestLoad_DecodesPostDefaultsAndInheritPost(t *testing.T) {
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
    post_defaults:
      - 'echo default'
      - {run: echo timed, timeout: 30s}
    statuses:
      - {name: a, kind: initial}
      - {name: b, kind: active}
      - {name: c, kind: terminal}
    transitions:
      - {from: a, to: b}
      - {from: b, to: c, inherit_post: false, post: ['echo own']}
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	typ := cfg.Types[0]
	if len(typ.PostDefaults) != 2 || typ.PostDefaults[0].Run != "echo default" || typ.PostDefaults[1].Timeout == 0 {
		t.Fatalf("PostDefaults not decoded: %+v", typ.PostDefaults)
	}
	if typ.Transitions[0].SkipPostDefaults {
		t.Fatal("edge without inherit_post must inherit (SkipPostDefaults=false)")
	}
	e := typ.Transitions[1]
	if !e.SkipPostDefaults || len(e.Post) != 1 || e.Post[0].Run != "echo own" {
		t.Fatalf("inherit_post:false edge = %+v", e)
	}
}
