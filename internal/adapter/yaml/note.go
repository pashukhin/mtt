package yaml

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// knowledgeDirName is the subdirectory of .mtt that holds one markdown file per note.
const knowledgeDirName = "knowledge"

// NoteStore is the YAML implementation of mtt.KnowledgeStore: one markdown file per
// note under .mtt/knowledge/<slug>.md (frontmatter + body). The slug is the file
// name; identity lives there, not in the frontmatter.
type NoteStore struct {
	root string
}

// NewKnowledgeStore returns a knowledge store rooted at the given project directory.
func NewKnowledgeStore(root string) *NoteStore { return &NoteStore{root: root} }

// dir is the notes directory (.mtt/knowledge).
func (s *NoteStore) dir() string { return filepath.Join(s.root, dirName, knowledgeDirName) }

// notePath builds the on-disk path for a slug, RE-VALIDATING it (defense in depth: a
// NoteSlug is a plain string type, so a raw cast could smuggle a traversal past the
// CLI's NewNoteSlug).
func (s *NoteStore) notePath(slug mtt.NoteSlug) (string, error) {
	if !slug.Valid() {
		return "", fmt.Errorf("invalid note slug %q", string(slug))
	}
	return filepath.Join(s.dir(), string(slug)+".md"), nil
}

// CreateNote reserves <slug>.md with O_CREATE|O_EXCL (no clobber, mirroring task
// mint), then atomically writes the content. An existing slug is an error (not
// ErrNotFound).
func (s *NoteStore) CreateNote(n mtt.Note) (mtt.Note, error) {
	path, err := s.notePath(n.Slug)
	if err != nil {
		return mtt.Note{}, err
	}
	dir := s.dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return mtt.Note{}, fmt.Errorf("create %s: %w", dir, err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return mtt.Note{}, fmt.Errorf("note slug %q already exists", string(n.Slug))
		}
		return mtt.Note{}, fmt.Errorf("reserve %s: %w", path, err)
	}
	_ = f.Close()
	return s.write(n, path)
}

// write serializes n and atomically persists it to path (temp+rename overwrites the
// reserved/existing file).
func (s *NoteStore) write(n mtt.Note, path string) (mtt.Note, error) {
	data, err := marshalNote(n)
	if err != nil {
		return mtt.Note{}, err
	}
	if err := atomicWrite(path, data); err != nil {
		return mtt.Note{}, err
	}
	return n, nil
}

// GetNote loads a note by slug, returning mtt.ErrNotFound when the file is absent.
func (s *NoteStore) GetNote(slug mtt.NoteSlug) (mtt.Note, error) {
	path, err := s.notePath(slug)
	if err != nil {
		return mtt.Note{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return mtt.Note{}, mtt.ErrNotFound
		}
		return mtt.Note{}, fmt.Errorf("read %s: %w", path, err)
	}
	note, err := parseNote(slug, data)
	if err != nil {
		return mtt.Note{}, fmt.Errorf("%s: %w", path, err)
	}
	return note, nil
}

// UpdateNote overwrites an existing note by n.Slug; missing note -> ErrNotFound.
func (s *NoteStore) UpdateNote(n mtt.Note) (mtt.Note, error) {
	path, err := s.notePath(n.Slug)
	if err != nil {
		return mtt.Note{}, err
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return mtt.Note{}, mtt.ErrNotFound
		}
		return mtt.Note{}, fmt.Errorf("stat %s: %w", path, err)
	}
	return s.write(n, path)
}

// DeleteNote removes .mtt/knowledge/<slug>.md; missing note -> ErrNotFound.
func (s *NoteStore) DeleteNote(slug mtt.NoteSlug) error {
	path, err := s.notePath(slug)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return mtt.ErrNotFound
		}
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

// ListNotes maps every .md file under .mtt/knowledge/ to the domain. A filename that
// is not a valid slug, or a file that fails to parse, is a load error (fail-fast — a
// hand-planted corrupt file). A missing directory yields an empty slice.
func (s *NoteStore) ListNotes() ([]mtt.Note, error) {
	dir := s.dir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	var notes []mtt.Note
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		slug, err := mtt.NewNoteSlug(strings.TrimSuffix(e.Name(), ".md"))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Join(dir, e.Name()), err)
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		note, err := parseNote(slug, data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		notes = append(notes, note)
	}
	return notes, nil
}
