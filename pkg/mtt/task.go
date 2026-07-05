package mtt

import "time"

// Task is a single unit of work. Field order == on-disk order (deterministic
// diff). Title is optional when Description is set (core requires at least one).
// Tags/DependsOn/Refs/Comments/History are reserved; they are populated in later
// sessions and omitted from storage while empty.
type Task struct {
	ID          TaskID
	Type        TypeName
	Title       string
	Status      string
	Parent      TaskID
	Tags        []string
	DependsOn   []TaskID
	Refs        []Ref
	Created     time.Time
	Updated     time.Time
	Description string
	Comments    []Comment
	History     []HistoryEntry
}

// RefKind is the closed vocabulary of reference targets — a value object.
type RefKind string

// The four reference kinds.
const (
	RefNote    RefKind = "note"
	RefTask    RefKind = "task"
	RefComment RefKind = "comment"
	RefURL     RefKind = "url"
)

// Valid reports whether k is one of the four defined kinds.
func (k RefKind) Valid() bool {
	switch k {
	case RefNote, RefTask, RefComment, RefURL:
		return true
	default:
		return false
	}
}

// Ref is a structured, verifiable reference (informational; not a blocking edge).
type Ref struct {
	Kind  RefKind
	ID    string
	Label string
}

// Comment is a tree node via nested Replies; ID is sequential within the task.
type Comment struct {
	ID      int
	Author  string
	Created time.Time
	Body    string
	Refs    []Ref
	Replies []Comment
}

// HistoryEntry is one append-only transition record. Role is the roles seam.
type HistoryEntry struct {
	At     time.Time
	By     string
	Role   string
	From   string
	To     string
	Checks []Check
}

// Check is one gate command's result recorded on a transition.
type Check struct {
	Cmd  string
	Exit int
}
