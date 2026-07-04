package yaml

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMint(t *testing.T) {
	root := t.TempDir()

	id, err := mint(root, "e")
	if err != nil || id != "e1" {
		t.Fatalf("first mint = %q, %v; want e1", id, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".mtt", "tasks", "e1.yaml")); err != nil {
		t.Fatalf("reserved file missing: %v", err)
	}
	if id, _ := mint(root, "e"); id != "e2" {
		t.Fatalf("second mint = %q, want e2", id)
	}
	if id, _ := mint(root, "t"); id != "t1" {
		t.Fatalf("independent prefix = %q, want t1", id)
	}

	// respects an existing higher number
	if err := os.WriteFile(filepath.Join(root, ".mtt", "tasks", "e9.yaml"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if id, _ := mint(root, "e"); id != "e10" {
		t.Fatalf("after e9 = %q, want e10", id)
	}
}
