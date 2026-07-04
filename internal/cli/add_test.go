package cli

import (
	"strings"
	"testing"
)

func runOut(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var out, errb strings.Builder
	root := NewRootCmd()
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), errb.String(), err
}

func TestAddCommand(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := runRoot(t, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	out, _, err := runOut(t, "add", "--type", "epic", "fix login")
	if err != nil {
		t.Fatalf("add epic: %v", err)
	}
	if !strings.Contains(out, "e1") {
		t.Fatalf("add output = %q, want it to mention e1", out)
	}

	// default type (task) requires a parent
	if _, _, err := runOut(t, "add", "just a task"); err == nil {
		t.Fatal("bare add of default type should error (needs parent)")
	}

	// --no-parent creates the default type at top level
	out, _, err = runOut(t, "add", "--no-parent", "orphan")
	if err != nil {
		t.Fatalf("add --no-parent: %v", err)
	}
	if !strings.Contains(out, "t1") {
		t.Fatalf("no-parent output = %q, want t1", out)
	}

	if _, _, err := runOut(t, "add", "--type", "ghost", "x"); err == nil {
		t.Fatal("unknown type should error")
	}
}

func TestAddArgError(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := runRoot(t, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	// an unquoted multi-word title arrives as several args — give a human-friendly hint.
	_, _, err := runOut(t, "add", "--no-parent", "fix", "login", "bug")
	if err == nil || !strings.Contains(err.Error(), "too many arguments") {
		t.Fatalf("multi-arg add should mention 'too many arguments', got: %v", err)
	}
}
