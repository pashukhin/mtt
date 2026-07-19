package yaml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInitLoadValidate(t *testing.T) {
	for _, name := range []string{"default", "coding", "hierarchy"} {
		root := t.TempDir()
		if err := Init(root, name, "demo", false); err != nil {
			t.Fatalf("init %s: %v", name, err)
		}
		cfg, s, err := Load(root)
		if err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("%s: domain invalid: %v", name, err)
		}
		if len(s.Prefixes) != len(cfg.Types) {
			t.Fatalf("%s: %d prefixes for %d types", name, len(s.Prefixes), len(cfg.Types))
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

func TestLoadCommandTimeoutDefault(t *testing.T) {
	root := t.TempDir()
	if err := Init(root, "default", "demo", false); err != nil {
		t.Fatalf("init: %v", err)
	}
	// remove the explicit command_timeout to prove the default kicks in
	writeConfigWithout(t, root, "command_timeout")
	_, s, err := Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if s.CommandTimeout != 5*time.Minute {
		t.Fatalf("timeout = %s, want 5m default", s.CommandTimeout)
	}
}

func TestLoadAuthorFromLocalOverlay(t *testing.T) {
	root := t.TempDir()
	if err := Init(root, "default", "demo", false); err != nil {
		t.Fatalf("init: %v", err)
	}
	overlay := filepath.Join(root, dirName, localConfigName)
	if err := os.WriteFile(overlay, []byte("author: grisha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, s, err := Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if s.Author != "grisha" {
		t.Fatalf("author = %q, want grisha", s.Author)
	}
}

func TestLoadCommandTimeoutFromConfig(t *testing.T) {
	root := t.TempDir()
	if err := Init(root, "default", "demo", false); err != nil {
		t.Fatalf("init: %v", err)
	}
	_, s, err := Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if s.CommandTimeout != 5*time.Minute { // the template sets command_timeout: 5m
		t.Fatalf("timeout = %s, want 5m from template", s.CommandTimeout)
	}
}

func TestLoadRequireParsed(t *testing.T) {
	root := t.TempDir()
	if err := Init(root, "default", "demo", false); err != nil {
		t.Fatalf("init: %v", err)
	}
	appendToConfig(t, root, "require:\n  who: true\n  why: true\n")
	_, s, err := Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !s.Require.Who || !s.Require.Why {
		t.Fatalf("Require = %+v, want both true", s.Require)
	}
}

func TestLoadRequireTightenOnly(t *testing.T) {
	root := t.TempDir()
	if err := Init(root, "default", "demo", false); err != nil {
		t.Fatalf("init: %v", err)
	}
	// committed requires who; local tries to relax who (must not) and add why (must).
	appendToConfig(t, root, "require:\n  who: true\n")
	overlay := filepath.Join(root, dirName, localConfigName)
	if err := os.WriteFile(overlay, []byte("require:\n  who: false\n  why: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, s, err := Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !s.Require.Who {
		t.Error("Who must stay true (a committed requirement cannot be relaxed by config.local)")
	}
	if !s.Require.Why {
		t.Error("Why must become true (config.local may tighten)")
	}
}

// appendToConfig appends raw YAML to .mtt/config.yaml (a new top-level block).
func appendToConfig(t *testing.T, root, block string) {
	t.Helper()
	path := filepath.Join(root, dirName, configName)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open config: %v", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString("\n" + block); err != nil {
		t.Fatalf("append config: %v", err)
	}
}

// writeConfigWithout rewrites .mtt/config.yaml dropping any line whose key is
// `key:` at column 0 — a crude way to simulate an absent top-level field.
func writeConfigWithout(t *testing.T, root, key string) {
	t.Helper()
	path := filepath.Join(root, dirName, configName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var kept []string
	for _, ln := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(ln, key+":") {
			continue
		}
		kept = append(kept, ln)
	}
	if err := os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}
