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
