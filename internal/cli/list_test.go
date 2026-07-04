package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func mustAdd(t *testing.T, args ...string) {
	t.Helper()
	if _, _, err := runOut(t, append([]string{"add"}, args...)...); err != nil {
		t.Fatalf("add %v: %v", args, err)
	}
}

func TestListCommand(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := runRoot(t, "init"); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, "--type", "epic", "build auth")
	mustAdd(t, "--type", "epic", "build billing")
	mustAdd(t, "--no-parent", "fix login") // default type (task) -> t1

	// presence, not order (wall-clock e2e/unit split)
	out, _, err := runOut(t, "list")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"e1  epic  [tbd]", "e2  epic  [tbd]", "t1  task  [tbd]"} {
		if !strings.Contains(out, want) {
			t.Fatalf("list missing %q in:\n%s", want, out)
		}
	}

	// --type task narrows to t1, drops epics
	out, _, err = runOut(t, "list", "--type", "task")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "t1  task") || strings.Contains(out, "e1  epic") {
		t.Fatalf("type=task filter wrong:\n%s", out)
	}

	// invalid --sort errors
	if _, _, err := runOut(t, "list", "--sort", "bogus"); err == nil {
		t.Fatal("invalid --sort should error")
	}

	// --json is a valid array of 3
	out, _, err = runOut(t, "list", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(out), &arr); err != nil {
		t.Fatalf("invalid json array: %v\n%s", err, out)
	}
	if len(arr) != 3 {
		t.Fatalf("json array len = %d, want 3", len(arr))
	}
}
