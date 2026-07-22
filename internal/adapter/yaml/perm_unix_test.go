//go:build !windows

package yaml

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestCreatePermsUmaskIndependent(t *testing.T) {
	// A freshly created task file must land the documented 0644 regardless of
	// the process umask (c18): atomicWrite's preserve-existing-mode must NOT
	// adopt the O_EXCL reserve artifact's umask-filtered mode — a zero-byte
	// target is "absent" on this path like on every other.
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)

	root := initHierarchy(t)
	created, err := NewTaskStore(root).Create(mtt.Task{Type: "task", Title: "A", Status: "tbd", Created: fixedTime(), Updated: fixedTime()})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	info, err := os.Stat(filepath.Join(root, ".mtt", "tasks", string(created.ID)+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Fatalf("fresh task file perm under umask 077 = %o, want 644", perm)
	}
}
