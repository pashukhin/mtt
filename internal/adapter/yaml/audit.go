package yaml

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// AuditStore is the append-only JSONL audit log at <root>/.mtt/audit.log.
type AuditStore struct{ root string }

// NewAuditStore wires the audit adapter for a project root.
func NewAuditStore(root string) *AuditStore { return &AuditStore{root: root} }

// auditLine is the on-disk JSON shape (keeps pkg/mtt free of json tags).
type auditLine struct {
	At     string `json:"at"`
	Who    string `json:"who,omitempty"`
	Why    string `json:"why,omitempty"`
	Action string `json:"action"`
	ID     string `json:"id"`
}

// Append writes one JSON line, creating .mtt if absent. Append-only (O_APPEND).
func (s *AuditStore) Append(e mtt.AuditEntry) error {
	dir := filepath.Join(s.root, ".mtt")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("audit: mkdir %s: %w", dir, err)
	}
	b, err := json.Marshal(auditLine{
		At:     e.At.UTC().Format(time.RFC3339),
		Who:    e.Who,
		Why:    e.Why,
		Action: e.Action,
		ID:     string(e.TaskID),
	})
	if err != nil {
		return fmt.Errorf("audit: marshal: %w", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "audit.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("audit: open log: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("audit: write: %w", err)
	}
	return nil
}
