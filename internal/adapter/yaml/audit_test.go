package yaml

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestAuditStore_AppendWritesJSONL(t *testing.T) {
	root := t.TempDir()
	s := NewAuditStore(root)
	at := time.Date(2026, 7, 11, 9, 20, 0, 0, time.UTC)

	if err := s.Append(mtt.AuditEntry{At: at, Who: "alice", Why: "stale dup", Action: "rm --force", TaskID: "t7"}); err != nil {
		t.Fatalf("append 1: %v", err)
	}
	if err := s.Append(mtt.AuditEntry{At: at, Who: "bob", Why: "bad import", Action: "rm --force", TaskID: "t9"}); err != nil {
		t.Fatalf("append 2: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(root, ".mtt", "audit.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	var lines []string
	for _, l := range strings.Split(string(raw), "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d: %q", len(lines), raw)
	}
	var got struct{ At, Who, Why, Action, ID string }
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("line 0 not JSON: %v", err)
	}
	if got.Who != "alice" || got.Why != "stale dup" || got.Action != "rm --force" || got.ID != "t7" || got.At != "2026-07-11T09:20:00Z" {
		t.Fatalf("line 0 fields wrong: %+v", got)
	}
}

func TestAuditStore_AppendCreatesMttDir(t *testing.T) {
	root := t.TempDir() // no .mtt yet
	if err := NewAuditStore(root).Append(mtt.AuditEntry{At: time.Unix(0, 0).UTC(), Action: "rm --force", TaskID: "t1"}); err != nil {
		t.Fatalf("append into fresh root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".mtt", "audit.log")); err != nil {
		t.Fatalf("log not created: %v", err)
	}
}
