package yaml

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitLoadValidate(t *testing.T) {
	for _, name := range []string{"default", "coding"} {
		root := t.TempDir()
		if err := Init(root, name, "demo", false); err != nil {
			t.Fatalf("init %s: %v", name, err)
		}
		cfg, prefixes, err := Load(root)
		if err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("%s: domain invalid: %v", name, err)
		}
		if len(prefixes) != len(cfg.Types) {
			t.Fatalf("%s: %d prefixes for %d types", name, len(prefixes), len(cfg.Types))
		}
		if got, ok := cfg.DefaultType(); !ok || got.Name == "" {
			t.Fatalf("%s: no default type", name)
		}
	}
}

func TestLoadOverlayOverridesName(t *testing.T) {
	root := t.TempDir()
	if err := Init(root, "default", "demo", false); err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(root, dirName, localConfigName)
	if err := os.WriteFile(overlay, []byte("project:\n  name: overridden\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Project.Name != "overridden" {
		t.Fatalf("name = %q, want overridden", cfg.Project.Name)
	}
}

func TestLoadMissing(t *testing.T) {
	if _, _, err := Load(t.TempDir()); err == nil {
		t.Fatal("want error loading a dir with no config")
	}
}
