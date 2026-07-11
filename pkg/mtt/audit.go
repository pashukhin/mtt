package mtt

import "time"

// AuditEntry records one out-of-flow dangerous action — a destruction that has no
// task history to carry its attribution.
type AuditEntry struct {
	At     time.Time
	Who    string // acting subject (--who/--by/MTT_BY/config.local author)
	Why    string // --why
	Action string // e.g. "rm --force"
	TaskID TaskID
}

// AuditStore appends dangerous-action records. Append-only; no read surface (t5).
type AuditStore interface {
	Append(AuditEntry) error
}
