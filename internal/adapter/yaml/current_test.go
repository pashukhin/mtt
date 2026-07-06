package yaml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func writeLocal(t *testing.T, root, body string) {
	t.Helper()
	dir := filepath.Join(root, dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, localConfigName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentUnsetIsNotAnError(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, dirName), 0o755); err != nil {
		t.Fatal(err)
	}
	id, ok, err := NewCurrent(root).Current()
	if err != nil || ok || id != "" {
		t.Fatalf("Current() on absent file = (%q,%v,%v), want (\"\",false,nil)", id, ok, err)
	}
}

func TestSetCurrentPreservesAuthorAndComments(t *testing.T) {
	root := t.TempDir()
	writeLocal(t, root, "# my identity\nauthor: alice\n")
	if err := NewCurrent(root).SetCurrent(mtt.TaskID("t5")); err != nil {
		t.Fatal(err)
	}
	id, ok, err := NewCurrent(root).Current()
	if err != nil || !ok || id != "t5" {
		t.Fatalf("Current() = (%q,%v,%v), want (t5,true,nil)", id, ok, err)
	}
	data, err := os.ReadFile(filepath.Join(root, dirName, localConfigName))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "author: alice") {
		t.Errorf("author not preserved:\n%s", s)
	}
	if !strings.Contains(s, "my identity") {
		t.Errorf("comment not preserved:\n%s", s)
	}
}

func TestSetOnAbsentFileThenClear(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, dirName), 0o755); err != nil {
		t.Fatal(err)
	}
	c := NewCurrent(root)
	if err := c.SetCurrent("t1"); err != nil {
		t.Fatal(err)
	}
	if id, ok, _ := c.Current(); !ok || id != "t1" {
		t.Fatalf("after set on absent file: (%q,%v)", id, ok)
	}
	if err := c.ClearCurrent(); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := c.Current(); ok {
		t.Fatal("ClearCurrent did not clear")
	}
}

func TestClearPreservesOtherKeys(t *testing.T) {
	root := t.TempDir()
	writeLocal(t, root, "author: bob\ncurrent: t9\n")
	if err := NewCurrent(root).ClearCurrent(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, dirName, localConfigName))
	if strings.Contains(string(data), "current") {
		t.Errorf("current not removed:\n%s", data)
	}
	if !strings.Contains(string(data), "author: bob") {
		t.Errorf("author lost:\n%s", data)
	}
}

func TestSetCurrentRejectsNonMappingRoot(t *testing.T) {
	root := t.TempDir()
	writeLocal(t, root, "- a\n- b\n") // a sequence-root config.local is corrupt
	if err := NewCurrent(root).SetCurrent("t1"); err == nil {
		t.Fatal("SetCurrent on a sequence-root config.local = nil, want an error (not silent corruption)")
	}
}
