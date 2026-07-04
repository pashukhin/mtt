package yaml

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFindRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, dirName), 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, start := range []string{root, nested} {
		got, err := FindRoot(start)
		if err != nil {
			t.Fatalf("FindRoot(%s): %v", start, err)
		}
		if got != root {
			t.Fatalf("FindRoot(%s) = %s, want %s", start, got, root)
		}
	}
	if _, err := FindRoot(t.TempDir()); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("FindRoot(uninit) err = %v, want ErrNotInitialized", err)
	}
}
