package mtt

import "testing"

func TestNewNoteSlug(t *testing.T) {
	valid := []string{"a", "auth", "auth-design", "a1", "kb-seed-2", "x9y8"}
	for _, s := range valid {
		if got, err := NewNoteSlug(s); err != nil || string(got) != s {
			t.Errorf("NewNoteSlug(%q) = (%q, %v); want (%q, nil)", s, got, err, s)
		}
		if !NoteSlug(s).Valid() {
			t.Errorf("NoteSlug(%q).Valid() = false; want true", s)
		}
	}
	invalid := []string{
		"",            // empty
		"Auth",        // uppercase
		"auth design", // space
		"-auth",       // leading hyphen
		"auth-",       // trailing hyphen
		"a--b",        // doubled hyphen
		"../x",        // traversal
		"a/b",         // path separator
		"/abs",        // absolute
		"auth.md",     // dot
		"a\nb",        // embedded newline
		"café",        // non-ASCII
	}
	for _, s := range invalid {
		if _, err := NewNoteSlug(s); err == nil {
			t.Errorf("NewNoteSlug(%q) = nil error; want rejection", s)
		}
		if NoteSlug(s).Valid() {
			t.Errorf("NoteSlug(%q).Valid() = true; want false", s)
		}
	}
}
