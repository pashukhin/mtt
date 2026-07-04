package cli

import (
	"strings"
	"testing"
)

func TestEditCommand(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := runRoot(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runOut(t, "add", "--type", "epic", "old title"); err != nil {
		t.Fatal(err)
	}

	if _, _, err := runOut(t, "edit", "e1", "--title", "new title"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	out, _, err := runOut(t, "show", "e1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "new title") {
		t.Fatalf("show after edit = %q", out)
	}

	// no editable flag -> error
	if _, _, err := runOut(t, "edit", "e1"); err == nil {
		t.Fatal("edit with no flag should error")
	}
	// missing id -> error
	if _, _, err := runOut(t, "edit", "nope", "--title", "x"); err == nil {
		t.Fatal("edit missing id should error")
	}
}
