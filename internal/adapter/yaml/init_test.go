package yaml

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func TestRenderGolden(t *testing.T) {
	for _, name := range []string{"default", "coding", "hierarchy"} {
		got, err := renderTemplate(name, "demo")
		if err != nil {
			t.Fatalf("render %s: %v", name, err)
		}
		golden := filepath.Join("testdata", "golden", name+".yaml")
		if *update {
			if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(golden, got, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("read golden %s (run -update first): %v", golden, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s render != golden", name)
		}
	}
}

func TestRenderUnknownTemplate(t *testing.T) {
	if _, err := renderTemplate("nope", "demo"); err == nil {
		t.Fatal("want error for unknown template")
	}
}

func TestInitWritesGitignore(t *testing.T) {
	root := t.TempDir()
	if err := Init(root, "default", "demo", false); err != nil {
		t.Fatalf("init: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".mtt", ".gitignore"))
	if err != nil {
		t.Fatalf("read .mtt/.gitignore: %v", err)
	}
	if !bytes.Contains(data, []byte("config.local.yaml")) {
		t.Fatalf(".mtt/.gitignore does not ignore config.local.yaml:\n%s", data)
	}
}

func TestInitHealsMissingGitignore(t *testing.T) {
	root := t.TempDir()
	if err := Init(root, "default", "demo", false); err != nil {
		t.Fatalf("init: %v", err)
	}
	gi := filepath.Join(root, ".mtt", ".gitignore")
	if err := os.Remove(gi); err != nil {
		t.Fatal(err)
	}
	if err := Init(root, "default", "demo", false); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("re-init err = %v, want ErrAlreadyInitialized", err)
	}
	if _, err := os.Stat(gi); err != nil {
		t.Fatalf("re-init did not heal the missing .gitignore: %v", err)
	}
}

func TestInitKeepsExistingGitignore(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".mtt")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	custom := []byte("# hand-authored\ncache/\n")
	gi := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(gi, custom, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Init(root, "default", "demo", false); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := Init(root, "coding", "demo", true); err != nil {
		t.Fatalf("force re-init: %v", err)
	}
	data, err := os.ReadFile(gi)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, custom) {
		t.Fatalf(".mtt/.gitignore clobbered:\n%s", data)
	}
}

func TestInit(t *testing.T) {
	root := t.TempDir()
	if err := Init(root, "default", "demo", false); err != nil {
		t.Fatalf("init: %v", err)
	}
	dst := filepath.Join(root, ".mtt", "config.yaml")
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if bytes.Contains(data, []byte("{{.Name}}")) || !bytes.Contains(data, []byte("name: demo")) {
		t.Fatalf("project name not substituted:\n%s", data)
	}
	if err := Init(root, "default", "demo", false); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("re-init err = %v, want ErrAlreadyInitialized", err)
	}
	if err := Init(root, "coding", "demo", true); err != nil {
		t.Fatalf("force re-init: %v", err)
	}
}
