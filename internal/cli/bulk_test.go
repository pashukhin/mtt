package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// bulkCmd builds a command with out/err buffers and the json/dry-run flags.
func bulkCmd() (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	c := &cobra.Command{Use: "x"}
	c.Flags().Bool("json", false, "")
	c.Flags().Bool("dry-run", false, "")
	var out, errb bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&errb)
	return c, &out, &errb
}

func TestRunBulkAllOk(t *testing.T) {
	c, out, _ := bulkCmd()
	err := runBulk(c, []mtt.TaskID{"t1", "t2"}, "tagged", func(mtt.TaskID) error { return nil })
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(out.String(), "tagged 2 task(s): t1, t2") {
		t.Fatalf("out = %q", out.String())
	}
}

func TestRunBulkPartialFailure(t *testing.T) {
	c, out, errb := bulkCmd()
	apply := func(id mtt.TaskID) error {
		if id == "t2" {
			return errors.New("boom")
		}
		return nil
	}
	err := runBulk(c, []mtt.TaskID{"t1", "t2"}, "tagged", apply)
	if err == nil || !strings.Contains(err.Error(), "1 of 2 task(s) failed") {
		t.Fatalf("aggregate err = %v", err)
	}
	// aggregate must NOT wrap a per-item error (exit-code safety)
	if errors.Is(err, mtt.ErrNotFound) {
		t.Fatal("aggregate must not wrap per-item errors")
	}
	if !strings.Contains(out.String(), "tagged 1 task(s): t1") {
		t.Fatalf("out = %q", out.String())
	}
	if !strings.Contains(errb.String(), "t2: boom") {
		t.Fatalf("stderr = %q", errb.String())
	}
}

func TestRunBulkAggregateNeverWrapsNotFound(t *testing.T) {
	c, _, _ := bulkCmd()
	apply := func(mtt.TaskID) error { return mtt.ErrNotFound }
	err := runBulk(c, []mtt.TaskID{"t1"}, "removed", apply)
	if err == nil || errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("err = %v; must be a plain aggregate", err)
	}
}

func TestRunBulkDryRun(t *testing.T) {
	c, out, errb := bulkCmd()
	_ = c.Flags().Set("dry-run", "true")
	called := false
	err := runBulk(c, []mtt.TaskID{"t1", "t2"}, "tagged", func(mtt.TaskID) error { called = true; return nil })
	if err != nil || called {
		t.Fatalf("dry-run must not apply; err=%v called=%v", err, called)
	}
	if out.String() != "t1\nt2\n" {
		t.Fatalf("dry-run out = %q", out.String())
	}
	if !strings.Contains(errb.String(), "would affect 2 task(s)") {
		t.Fatalf("dry-run stderr = %q", errb.String())
	}
}

func TestRunBulkJSON(t *testing.T) {
	c, out, _ := bulkCmd()
	_ = c.Flags().Set("json", "true")
	apply := func(id mtt.TaskID) error {
		if id == "t2" {
			return errors.New("boom")
		}
		return nil
	}
	_ = runBulk(c, []mtt.TaskID{"t1", "t2"}, "tagged", apply)
	s := out.String()
	if !strings.Contains(s, `"id": "t1"`) || !strings.Contains(s, `"status": "tagged"`) ||
		!strings.Contains(s, `"status": "error"`) || !strings.Contains(s, `"error": "boom"`) {
		t.Fatalf("json = %s", s)
	}
}

func TestRunBulkEmptyIsNoOp(t *testing.T) {
	c, out, _ := bulkCmd()
	if err := runBulk(c, nil, "tagged", func(mtt.TaskID) error { return errors.New("x") }); err != nil {
		t.Fatalf("empty set err: %v", err)
	}
	if strings.TrimSpace(out.String()) != "" {
		t.Fatalf("empty out = %q", out.String())
	}
}
