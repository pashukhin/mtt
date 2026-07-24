package yaml

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_DecodesEvents(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".mtt")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `version: 1
project: {name: demo}
events:
  task:
    create:
      post: ['echo task-created {{.ID}} {{.Event}}']
    delete:
      post: [{run: echo bye, timeout: 10s}]
  note:
    update:
      post: ['echo note {{.Slug}}']
types:
  - name: task
    prefix: t
    default: true
    statuses:
      - {name: a, kind: initial}
      - {name: b, kind: active}
      - {name: c, kind: terminal}
    transitions:
      - {from: a, to: b}
      - {from: b, to: c}
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := cfg.Events.Task.Create.Post; len(got) != 1 || got[0].Run != "echo task-created {{.ID}} {{.Event}}" {
		t.Fatalf("task.create = %+v", got)
	}
	if got := cfg.Events.Task.Delete.Post; len(got) != 1 || got[0].Timeout != 10*time.Second {
		t.Fatalf("task.delete = %+v", got)
	}
	if got := cfg.Events.Task.Update.Post; len(got) != 0 {
		t.Fatalf("unconfigured task.update = %+v, want empty", got)
	}
	if got := cfg.Events.Note.Update.Post; len(got) != 1 || got[0].Run != "echo note {{.Slug}}" {
		t.Fatalf("note.update = %+v", got)
	}
}
