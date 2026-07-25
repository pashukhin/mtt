package scaffold

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunCreatesThenIdempotent(t *testing.T) {
	root := t.TempDir()
	res, err := Run(root, Registry())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res) != 1 || res[0].Action != Created {
		t.Fatalf("first run = %+v, want one Created", res)
	}
	path := filepath.Join(root, ".claude", "settings.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("settings not written: %v", err)
	}
	res2, err := Run(root, Registry())
	if err != nil {
		t.Fatalf("Run(2): %v", err)
	}
	if res2[0].Action != Unchanged {
		t.Fatalf("second run = %q, want Unchanged", res2[0].Action)
	}
}

func TestRunPropagatesNonNotExistReadError(t *testing.T) {
	// r6/d3: a .claude/settings.json that is a DIRECTORY is a non-IsNotExist read
	// error and must propagate, not be mistaken for absent.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude", "settings.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(root, Registry()); err == nil {
		t.Fatal("expected a read error to propagate")
	}
}
