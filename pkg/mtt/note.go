package mtt

import (
	"errors"
	"fmt"
	"regexp"
	"time"
)

// NoteSlug is a knowledge-note identity. Unlike the opaque TaskID/TypeName/
// StatusName (which reject empty but never parse structure), a NoteSlug is
// STRUCTURALLY validated: it is the note's file name (.mtt/knowledge/<slug>.md), so
// it must be a safe path segment. This is a deliberate, documented exception to the
// opaque-identity rule (see pkg/mtt/CLAUDE.md) — the regex is the traversal defense.
type NoteSlug string

// noteSlugRe is the kebab-ASCII slug shape: lowercase letters/digits in
// hyphen-separated groups. No '/', '.', whitespace, uppercase, non-ASCII, or
// leading/trailing/doubled '-'. So "../x", "a/b", "/abs", "Foo", "a--b" are rejected.
var noteSlugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// NewNoteSlug returns a validated NoteSlug, rejecting empty and any non-kebab-ASCII
// string (the file-name / traversal guard). Unlike NewTaskID it DOES parse structure
// on purpose — a slug is a path segment, not a provider-minted opaque id.
func NewNoteSlug(s string) (NoteSlug, error) {
	if s == "" {
		return "", errors.New("mtt: empty note slug")
	}
	if !noteSlugRe.MatchString(s) {
		return "", fmt.Errorf("mtt: invalid note slug %q (use lowercase letters, digits, and single hyphens)", s)
	}
	return NoteSlug(s), nil
}

// Valid reports whether s is a well-formed slug (non-empty, kebab-ASCII).
func (s NoteSlug) Valid() bool { return s != "" && noteSlugRe.MatchString(string(s)) }

// Note is a knowledge-base entry: a markdown Body plus metadata. Identity is Slug
// (the on-disk file name). In the seed a note is single-version (its history is git);
// Version/Predecessor are deferred to t6. Refs on notes are deferred to t1 — the seed
// note is refs-free.
type Note struct {
	Slug    NoteSlug
	Title   string
	Tags    []string
	Body    string
	Created time.Time
	Updated time.Time
}
