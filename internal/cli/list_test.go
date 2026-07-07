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

func TestListSortPriorityErrorText(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := runRoot(t, "init"); err != nil {
		t.Fatal(err)
	}
	// the --sort error text now includes priority
	_, _, err := runOut(t, "list", "--sort", "bogus")
	if err == nil || !strings.Contains(err.Error(), "created|updated|priority") {
		t.Fatalf("err = %v, want mention of created|updated|priority", err)
	}
	// --sort priority is accepted
	if _, _, err := runOut(t, "list", "--sort", "priority"); err != nil {
		t.Fatalf("--sort priority should be accepted: %v", err)
	}
}

func TestListPriorityFilterAndSort(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := runRoot(t, "init"); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, "--no-parent", "--priority", "low", "t1 low")
	mustAdd(t, "--no-parent", "--priority", "high", "t2 high")
	mustAdd(t, "--no-parent", "--priority", "medium", "t3 med")

	// --sort priority puts the (unique) high task first
	out, _, err := runOut(t, "list", "--sort", "priority")
	if err != nil {
		t.Fatal(err)
	}
	if idx := strings.Index(out, "t2 high"); idx < 0 || strings.Index(out, "t1 low") < idx {
		t.Fatalf("--sort priority should put high (t2) before low (t1):\n%s", out)
	}

	// --priority high filters to just the high task
	out, _, err = runOut(t, "list", "--priority", "high")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "t2 high") || strings.Contains(out, "t1 low") {
		t.Fatalf("--priority high filter wrong:\n%s", out)
	}

	// an invalid --priority filter value errors
	if _, _, err := runOut(t, "list", "--priority", "urgent"); err == nil {
		t.Fatal("invalid --priority should error")
	}
}
