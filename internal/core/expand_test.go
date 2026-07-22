package core

import (
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestExpandCommands(t *testing.T) {
	ctx := cmdContext{ID: "t1", Type: "task", From: "tbd", To: "in_progress"}
	out, err := expandCommands([]mtt.Command{{Run: "git checkout -b task/{{.ID}}", Timeout: 5 * time.Second}}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Run != "git checkout -b task/t1" {
		t.Fatalf("run = %q, want git checkout -b task/t1", out[0].Run)
	}
	if out[0].Timeout != 5*time.Second {
		t.Fatalf("timeout dropped: %v", out[0].Timeout)
	}
}

func TestExpandCommandsAllFields(t *testing.T) {
	ctx := cmdContext{ID: "t1", Type: "task", From: "tbd", To: "in_progress"}
	out, err := expandCommands([]mtt.Command{{Run: "echo {{.From}} {{.To}} {{.Type}} {{.ID}}"}}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Run != "echo tbd in_progress task t1" {
		t.Fatalf("run = %q", out[0].Run)
	}
}

func TestExpandCommandsUnknownField(t *testing.T) {
	if _, err := expandCommands([]mtt.Command{{Run: "echo {{.Title}}"}}, cmdContext{}); err == nil {
		t.Fatal("want an error for an unexposed field {{.Title}}")
	}
}

func TestExpandCommandsMalformed(t *testing.T) {
	if _, err := expandCommands([]mtt.Command{{Run: "echo {{.ID"}}, cmdContext{}); err == nil {
		t.Fatal("want a parse error for a malformed template")
	}
}

func TestExpandCommandsExpandsRollback(t *testing.T) {
	ctx := cmdContext{ID: "t1", Type: "task", From: "tbd", To: "in_progress"}
	out, err := expandCommands([]mtt.Command{{
		Run:      "git checkout -b task/{{.ID}}",
		Rollback: &mtt.Command{Run: "git branch -D task/{{.ID}}", Timeout: 5 * time.Second},
	}}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Rollback == nil || out[0].Rollback.Run != "git branch -D task/t1" {
		t.Fatalf("rollback run = %+v, want git branch -D task/t1", out[0].Rollback)
	}
	if out[0].Rollback.Timeout != 5*time.Second {
		t.Fatalf("rollback timeout dropped: %v", out[0].Rollback.Timeout)
	}
}

func TestExpandCommandsMalformedRollback(t *testing.T) {
	_, err := expandCommands([]mtt.Command{{
		Run:      "true",
		Rollback: &mtt.Command{Run: "echo {{.Title}}"}, // unexposed field → error up-front
	}}, cmdContext{ID: "t1"})
	if err == nil {
		t.Fatal("want an error for a malformed rollback template (before any run)")
	}
}

func TestExpandCommandsNilRollbackStaysNil(t *testing.T) {
	out, err := expandCommands([]mtt.Command{{Run: "true"}}, cmdContext{})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Rollback != nil {
		t.Fatal("nil rollback became non-nil")
	}
}

func TestExpandText(t *testing.T) {
	cases := []struct{ raw, id, typ, from, to, want string }{
		{"task/{{.ID}}", "t17", "task", "tbd", "in_progress", "task/t17"},
		{"{{.From}}→{{.To}} ({{.Type}})", "t1", "task", "tbd", "done", "tbd→done (task)"},
		{"no placeholders", "t1", "task", "a", "b", "no placeholders"},
		{"", "t1", "task", "a", "b", ""},
		{"{{.Title}}", "t1", "task", "a", "b", "{{.Title}}"}, // unknown field -> raw (best-effort)
		{"{{.ID", "t1", "task", "a", "b", "{{.ID"},           // malformed -> raw
	}
	for _, c := range cases {
		if got := ExpandText(c.raw, c.id, c.typ, c.from, c.to); got != c.want {
			t.Fatalf("ExpandText(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}
