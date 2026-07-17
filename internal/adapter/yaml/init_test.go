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
