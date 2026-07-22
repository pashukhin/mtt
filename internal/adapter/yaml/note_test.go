package yaml

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

var _ mtt.KnowledgeStore = (*NoteStore)(nil)

func TestNoteStoreCRUD(t *testing.T) {
	root := t.TempDir()
	s := NewKnowledgeStore(root)
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	// Create writes the file.
	if _, err := s.CreateNote(mtt.Note{Slug: "auth-design", Title: "Auth", Tags: []string{"design"}, Body: "b\n", Created: ts, Updated: ts}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".mtt", "knowledge", "auth-design.md")); err != nil {
		t.Fatalf("file not written: %v", err)
	}

	// Get round-trips.
	got, err := s.GetNote("auth-design")
	if err != nil || got.Title != "Auth" || got.Body != "b\n" {
		t.Fatalf("get: %+v, %v", got, err)
	}

	// Create refuses an existing slug (no clobber), and does NOT overwrite the body.
	if _, err := s.CreateNote(mtt.Note{Slug: "auth-design", Body: "clobber", Created: ts, Updated: ts}); err == nil {
		t.Fatal("create existing slug: want error")
	}
	if again, _ := s.GetNote("auth-design"); again.Body != "b\n" {
		t.Fatalf("clobbered: body = %q", again.Body)
	}

	// Update overwrites.
	if _, err := s.UpdateNote(mtt.Note{Slug: "auth-design", Title: "Auth v2", Body: "b2\n", Created: ts, Updated: ts.Add(time.Hour)}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got, _ := s.GetNote("auth-design"); got.Title != "Auth v2" {
		t.Fatalf("update not applied: %+v", got)
	}

	// List returns it.
	notes, err := s.ListNotes()
	if err != nil || len(notes) != 1 {
		t.Fatalf("list: %d notes, %v", len(notes), err)
	}

	// Delete removes it.
	if err := s.DeleteNote("auth-design"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.GetNote("auth-design"); !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("get after delete: want ErrNotFound, got %v", err)
	}
}

func TestNoteStoreNotFoundAndCorrupt(t *testing.T) {
	root := t.TempDir()
	s := NewKnowledgeStore(root)

	if _, err := s.GetNote("missing"); !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("get missing: want ErrNotFound, got %v", err)
	}
	if err := s.DeleteNote("missing"); !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("delete missing: want ErrNotFound, got %v", err)
	}
	if _, err := s.UpdateNote(mtt.Note{Slug: "missing"}); !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("update missing: want ErrNotFound, got %v", err)
	}

	// A corrupt file (no frontmatter) is a load error, NOT ErrNotFound.
	dir := filepath.Join(root, ".mtt", "knowledge")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.md"), []byte("no header\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetNote("bad"); err == nil || errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("get corrupt: want a non-ErrNotFound error, got %v", err)
	}
	if _, err := s.ListNotes(); err == nil {
		t.Fatal("list with a corrupt file: want error")
	}
}

func TestNoteStoreRejectsTraversalSlug(t *testing.T) {
	s := NewKnowledgeStore(t.TempDir())
	// A raw NoteSlug cast bypasses NewNoteSlug; every path-building method must
	// re-validate so a traversal never reaches the filesystem.
	if _, err := s.GetNote(mtt.NoteSlug("../evil")); err == nil {
		t.Fatal("GetNote traversal slug: want rejection")
	}
	if err := s.DeleteNote(mtt.NoteSlug("../evil")); err == nil {
		t.Fatal("DeleteNote traversal slug: want rejection")
	}
	if _, err := s.CreateNote(mtt.Note{Slug: mtt.NoteSlug("../evil")}); err == nil {
		t.Fatal("CreateNote traversal slug: want rejection")
	}
}

func TestNoteReserveArtifactIsInvisible(t *testing.T) {
	root := t.TempDir()
	s := NewKnowledgeStore(root)
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if _, err := s.CreateNote(mtt.Note{Slug: "real", Body: "x\n", Created: ts, Updated: ts}); err != nil {
		t.Fatalf("create: %v", err)
	}
	// A zero-byte note file is CreateNote's reserve window artifact (a crash
	// between the O_EXCL reserve and the content write, c18 — mirroring mint) —
	// reads must skip it instead of fail-stopping the whole store.
	if err := os.WriteFile(filepath.Join(root, ".mtt", "knowledge", "ghost.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	notes, err := s.ListNotes()
	if err != nil {
		t.Fatalf("ListNotes must skip a zero-byte reserve artifact, got: %v", err)
	}
	if len(notes) != 1 || notes[0].Slug != "real" {
		t.Fatalf("ListNotes = %v, want just real", notes)
	}
	if _, err := s.GetNote("ghost"); !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("GetNote on a reserve artifact = %v; want ErrNotFound", err)
	}
}

func TestNoteStoreListRejectsInvalidFilename(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".mtt", "knowledge")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A file whose NAME is not a valid slug is a load error (the filename guard),
	// even though its CONTENT is well-formed — distinct from ErrNotFound.
	good := []byte("---\ncreated: \"2026-01-02T03:04:05Z\"\nupdated: \"2026-01-02T03:04:05Z\"\n---\nx\n")
	if err := os.WriteFile(filepath.Join(dir, "Upper.md"), good, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := NewKnowledgeStore(root).ListNotes()
	if err == nil || errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("ListNotes with an invalid-slug filename: want a non-ErrNotFound error, got %v", err)
	}
}
