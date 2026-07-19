package mtt

// KnowledgeStore is the mandatory-minimum driven port for knowledge notes — the
// second independent store (like Confluence atop Jira). Implementations map their own
// DTOs to and from Note. The base port has NO versioning (a note has one current
// version; history is git) and NO search — those are later optional capabilities.
type KnowledgeStore interface {
	// CreateNote persists a new note; the slug must be free — an existing slug
	// yields an error (not ErrNotFound), never a silent overwrite.
	CreateNote(n Note) (Note, error)
	// GetNote loads a note by slug, returning ErrNotFound when it does not resolve.
	GetNote(slug NoteSlug) (Note, error)
	// ListNotes returns all notes; order unspecified — callers impose their own.
	ListNotes() ([]Note, error)
	// UpdateNote overwrites an existing note by n.Slug; missing note -> ErrNotFound.
	UpdateNote(n Note) (Note, error)
	// DeleteNote removes a note by slug; missing note -> ErrNotFound.
	DeleteNote(slug NoteSlug) error
}
