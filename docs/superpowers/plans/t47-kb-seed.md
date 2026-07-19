# KB seed — notes CRUD Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a minimal knowledge base — create/list/show/edit/remove markdown notes with stable slugs — so `t1` (references) can point a `note` ref at a real target and project knowledge can be dog-fooded into `.mtt/knowledge/`.

**Architecture:** Hexagonal, mirroring the task half. New pure domain (`mtt.Note`, `mtt.NoteSlug`, `mtt.KnowledgeStore` port) in `pkg/mtt`; a YAML driven adapter storing one `.mtt/knowledge/<slug>.md` file (YAML frontmatter + verbatim markdown body) with a precise serialization contract; `core` usecases (add/edit) + a pure list filter; a thin `mtt note` cobra command group. Refs, full-text search, and note versioning are explicitly out (t1 / t6).

**Tech Stack:** Go 1.23, `gopkg.in/yaml.v3`, cobra, `go-internal/testscript` (e2e). Storage: YAML file-per-note.

## Global Constraints

- **TDD** (red → green → refactor); `make check` green before every commit.
- **Hexagonal / layering:** `cli → core → port ← adapter`; `core` imports only `pkg/mtt` (never `adapter/*`); the CLI assembles the YAML `KnowledgeStore` at the composition root and injects it. Storage only through the port.
- **DDD:** `pkg/mtt` carries no yaml/json tags; the adapter maps via a DTO. Clock is injected (`now func() time.Time`) into `core` mutations.
- **Determinism:** frontmatter is a **struct** DTO (fixed field order); the body is written **byte-for-byte** (trailing newline preserved). Never `yaml.Unmarshal` the whole `.md` file.
- **Slug safety:** `NoteSlug` is `^[a-z0-9]+(-[a-z0-9]+)*$`, validated at the CLI boundary via `NewNoteSlug` AND re-validated in every adapter path-building method AND on load (filename-derived). No path is built from an unvalidated slug.
- **Reuse:** tags via `core.canonicalTags` + `mtt.NormalizeTag` (CLI `toTags`); list filter via `anyOrEmptyIntersect`; atomic write via the adapter's existing `atomicWrite`; reserve via `O_CREATE|O_EXCL` (mirroring task `mint`).
- **No new heavy dependencies.** Commit trailer: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`.

---

### Task 1: `pkg/mtt` domain — `NoteSlug`, `Note`, `KnowledgeStore`

**Files:**
- Create: `pkg/mtt/note.go`
- Create: `pkg/mtt/knowledge.go`
- Modify: `pkg/mtt/store.go:26` (generalize the `ErrNotFound` doc comment)
- Test: `pkg/mtt/note_test.go`

**Interfaces:**
- Produces: `mtt.NoteSlug` (string type); `mtt.NewNoteSlug(string) (NoteSlug, error)`; `mtt.NoteSlug.Valid() bool`; `mtt.Note{Slug NoteSlug; Title string; Tags []string; Body string; Created, Updated time.Time}`; `mtt.KnowledgeStore` interface (`CreateNote/GetNote/ListNotes/UpdateNote/DeleteNote`).
- Consumes: `mtt.ErrNotFound` (existing).

- [ ] **Step 1: Write the failing slug test.** Create `pkg/mtt/note_test.go`:

```go
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
```

- [ ] **Step 2: Run it, verify it fails.**

Run: `go test ./pkg/mtt/ -run TestNewNoteSlug`
Expected: FAIL (undefined `NewNoteSlug` / `NoteSlug`).

- [ ] **Step 3: Implement `note.go`.** Create `pkg/mtt/note.go`:

```go
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
```

- [ ] **Step 4: Implement `knowledge.go`.** Create `pkg/mtt/knowledge.go`:

```go
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
```

- [ ] **Step 5: Generalize the `ErrNotFound` doc comment.** In `pkg/mtt/store.go`, replace the comment on line 26:

```go
// ErrNotFound is returned by TaskStore.Get when the ID does not resolve.
```

with:

```go
// ErrNotFound is returned by TaskStore.Get and KnowledgeStore.GetNote when the
// ID/slug does not resolve. (The message text is task-worded; the CLI surfaces the
// right noun via taskNotFound/noteNotFound wrappers.)
```

- [ ] **Step 6: Run the test, verify it passes.**

Run: `go test ./pkg/mtt/ -run TestNewNoteSlug -v`
Expected: PASS.

- [ ] **Step 7: Gate + commit.**

Run: `make check`
Expected: `OK`.

```bash
git add pkg/mtt/note.go pkg/mtt/knowledge.go pkg/mtt/store.go pkg/mtt/note_test.go
git commit -m "t47: pkg/mtt — Note, NoteSlug (validated), KnowledgeStore port

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: YAML adapter — note serialization contract (`marshalNote` / `parseNote`)

**Files:**
- Create: `internal/adapter/yaml/note_dto.go`
- Test: `internal/adapter/yaml/note_dto_test.go`
- Create (via `-update`): `internal/adapter/yaml/testdata/golden/note_min.md`, `note_full.md`

**Interfaces:**
- Consumes: `mtt.Note`, `mtt.NoteSlug` (Task 1); the package `atomicWrite`/`dirName` (existing).
- Produces: `ymlNote` (frontmatter DTO); `marshalNote(mtt.Note) ([]byte, error)`; `parseNote(mtt.NoteSlug, []byte) (mtt.Note, error)`. These are pure (no I/O) — the file store (Task 3) calls them.

- [ ] **Step 1: Write the failing round-trip + edge tests.** Create `internal/adapter/yaml/note_dto_test.go`:

```go
package yaml

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func fixedNote() mtt.Note {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return mtt.Note{Slug: "auth-design", Title: "Auth design", Tags: []string{"auth", "design"}, Body: "First body.\n", Created: ts, Updated: ts}
}

func TestNoteRoundTrip(t *testing.T) {
	cases := map[string]mtt.Note{
		"full":            fixedNote(),
		"minimal":         {Slug: "stub", Created: time.Unix(10, 0).UTC(), Updated: time.Unix(10, 0).UTC()},
		"body has ---":    withBody(fixedNote(), "intro\n\n---\n\nafter a thematic break\n"),
		"no trailing nl":  withBody(fixedNote(), "no newline at end"),
		"body starts ---": withBody(fixedNote(), "---\nleading break\n"),
		"empty body":      withBody(fixedNote(), ""),
	}
	for name, in := range cases {
		data, err := marshalNote(in)
		if err != nil {
			t.Fatalf("%s: marshal: %v", name, err)
		}
		got, err := parseNote(in.Slug, data)
		if err != nil {
			t.Fatalf("%s: parse: %v", name, err)
		}
		if got.Body != in.Body {
			t.Errorf("%s: body round-trip: got %q want %q", name, got.Body, in.Body)
		}
		if got.Title != in.Title || strings.Join(got.Tags, ",") != strings.Join(in.Tags, ",") {
			t.Errorf("%s: meta round-trip: got %+v want %+v", name, got, in)
		}
		if !got.Created.Equal(in.Created) || !got.Updated.Equal(in.Updated) {
			t.Errorf("%s: time round-trip: got %v/%v want %v/%v", name, got.Created, got.Updated, in.Created, in.Updated)
		}
	}
}

func withBody(n mtt.Note, body string) mtt.Note { n.Body = body; return n }

func TestParseNoteCorruptNotFound(t *testing.T) {
	// A file that does not begin with "---" is corrupt — an error, and crucially NOT
	// mtt.ErrNotFound (so the store's absent-file exit-4 stays distinct).
	_, err := parseNote("x", []byte("no frontmatter here\n"))
	if err == nil {
		t.Fatal("parseNote on a headerless file: want error")
	}
	if errors.Is(err, mtt.ErrNotFound) {
		t.Fatal("corrupt-parse error must NOT be ErrNotFound")
	}
	// Unterminated frontmatter is also an error.
	if _, err := parseNote("x", []byte("---\ntitle: x\n")); err == nil {
		t.Fatal("parseNote on unterminated frontmatter: want error")
	}
}

func TestNoteGolden(t *testing.T) {
	for _, tc := range []struct {
		name string
		note mtt.Note
		file string
	}{
		{"min", mtt.Note{Slug: "stub", Created: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), Updated: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}, "note_min.md"},
		{"full", fixedNote(), "note_full.md"},
	} {
		got, err := marshalNote(tc.note)
		if err != nil {
			t.Fatalf("%s: marshal: %v", tc.name, err)
		}
		golden := filepath.Join("testdata", "golden", tc.file)
		if *update {
			if err := os.WriteFile(golden, got, 0o644); err != nil {
				t.Fatalf("write golden: %v", err)
			}
			continue
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("read golden (run -update first): %v", err)
		}
		if string(got) != string(want) {
			t.Errorf("%s serialization != golden:\n%s", tc.name, got)
		}
	}
}
```

(Note: `*update` is the package's existing golden flag — see `task_dto_test.go`.)

- [ ] **Step 2: Run it, verify it fails.**

Run: `go test ./internal/adapter/yaml/ -run 'TestNote'`
Expected: FAIL (undefined `marshalNote`/`parseNote`).

- [ ] **Step 3: Implement `note_dto.go`.** Create `internal/adapter/yaml/note_dto.go`:

```go
package yaml

import (
	"bytes"
	"fmt"
	"time"

	goyaml "gopkg.in/yaml.v3"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// ymlNote is the YAML frontmatter DTO for a note. A struct (not a map) so field
// order is deterministic. The slug is NOT here — it is the file name (single source
// of truth for identity). created/updated are always present (frontmatter is never
// empty), which is what makes the "first closing ---" read rule unambiguous.
type ymlNote struct {
	Title   string   `yaml:"title,omitempty"`
	Tags    []string `yaml:"tags,omitempty"`
	Created string   `yaml:"created"`
	Updated string   `yaml:"updated"`
}

// noteDelim is the frontmatter delimiter line (with its newline).
const noteDelim = "---\n"

// marshalNote serializes a note to the on-disk hybrid document: "---\n" +
// struct-ordered YAML frontmatter + "---\n" + the body VERBATIM (its bytes, incl.
// any "---" lines and its trailing-newline state, are preserved exactly).
func marshalNote(n mtt.Note) ([]byte, error) {
	fm, err := goyaml.Marshal(ymlNote{
		Title:   n.Title,
		Tags:    n.Tags,
		Created: n.Created.UTC().Format(time.RFC3339),
		Updated: n.Updated.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal note %s: %w", n.Slug, err)
	}
	var b bytes.Buffer
	b.WriteString(noteDelim)
	b.Write(fm)
	b.WriteString(noteDelim)
	b.WriteString(n.Body)
	return b.Bytes(), nil
}

// parseNote splits a note file into frontmatter + body and maps to the domain. The
// file MUST begin with "---\n"; the frontmatter runs up to the FIRST subsequent line
// that is exactly "---"; the body is everything after that delimiter, byte-for-byte
// (so "---" inside the body is preserved — only the first closing delimiter counts).
// Only the frontmatter bytes are unmarshaled — never the whole file ("\n---\n" is
// yaml's document separator). slug is the (already-validated) file name.
func parseNote(slug mtt.NoteSlug, data []byte) (mtt.Note, error) {
	if !bytes.HasPrefix(data, []byte(noteDelim)) {
		return mtt.Note{}, fmt.Errorf("note %s: missing frontmatter (no leading ---)", slug)
	}
	rest := data[len(noteDelim):]
	idx := bytes.Index(rest, []byte("\n"+noteDelim)) // the closing delimiter line
	if idx < 0 {
		return mtt.Note{}, fmt.Errorf("note %s: unterminated frontmatter (no closing ---)", slug)
	}
	fmBytes := rest[:idx+1] // include the last frontmatter line's trailing newline
	body := rest[idx+1+len(noteDelim):]
	var yn ymlNote
	if err := goyaml.Unmarshal(fmBytes, &yn); err != nil {
		return mtt.Note{}, fmt.Errorf("note %s: parse frontmatter: %w", slug, err)
	}
	created, err := time.Parse(time.RFC3339, yn.Created)
	if err != nil {
		return mtt.Note{}, fmt.Errorf("note %s: created: %w", slug, err)
	}
	updated, err := time.Parse(time.RFC3339, yn.Updated)
	if err != nil {
		return mtt.Note{}, fmt.Errorf("note %s: updated: %w", slug, err)
	}
	return mtt.Note{Slug: slug, Title: yn.Title, Tags: yn.Tags, Body: string(body), Created: created, Updated: updated}, nil
}
```

- [ ] **Step 4: Run tests + generate goldens.**

Run: `go test ./internal/adapter/yaml/ -run 'TestNoteRoundTrip|TestParseNoteCorruptNotFound'`
Expected: PASS.
Run: `go test ./internal/adapter/yaml/ -run TestNoteGolden -update`
Then inspect `git diff internal/adapter/yaml/testdata/golden/note_*.md` — confirm `note_min.md` has only `created`/`updated` between the `---` lines (no `title`/`tags`) and `note_full.md` shows the fixed field order + `First body.` after the closing `---`.
Run: `go test ./internal/adapter/yaml/ -run TestNoteGolden`
Expected: PASS.

- [ ] **Step 5: Gate + commit.**

Run: `make check`

```bash
git add internal/adapter/yaml/note_dto.go internal/adapter/yaml/note_dto_test.go internal/adapter/yaml/testdata/golden/note_min.md internal/adapter/yaml/testdata/golden/note_full.md
git commit -m "t47: yaml — note serialization contract (frontmatter + verbatim body)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: YAML adapter — `KnowledgeStore` file store

**Files:**
- Create: `internal/adapter/yaml/note.go`
- Test: `internal/adapter/yaml/note_test.go`

**Interfaces:**
- Consumes: `marshalNote`/`parseNote` (Task 2); `atomicWrite`/`dirName` (existing); `mtt.Note`/`mtt.NoteSlug`/`mtt.NewNoteSlug`/`mtt.ErrNotFound` (Task 1).
- Produces: `yaml.NewKnowledgeStore(root string) *NoteStore`; `*NoteStore` implements `mtt.KnowledgeStore`; `knowledgeDirName = "knowledge"`.

- [ ] **Step 1: Write the failing store test.** Create `internal/adapter/yaml/note_test.go`:

```go
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
```

- [ ] **Step 2: Run it, verify it fails.**

Run: `go test ./internal/adapter/yaml/ -run TestNoteStore`
Expected: FAIL (undefined `NewKnowledgeStore`/`NoteStore`).

- [ ] **Step 3: Implement `note.go`.** Create `internal/adapter/yaml/note.go`:

```go
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

// notePath builds the on-disk path for a slug, RE-VALIDATING it (defense in depth: a
// NoteSlug is a plain string type, so a raw cast could smuggle a traversal past the
// CLI's NewNoteSlug).
func (s *NoteStore) notePath(slug mtt.NoteSlug) (string, error) {
	if !slug.Valid() {
		return "", fmt.Errorf("invalid note slug %q", string(slug))
	}
	return filepath.Join(s.root, dirName, knowledgeDirName, string(slug)+".md"), nil
}

// CreateNote reserves <slug>.md with O_CREATE|O_EXCL (no clobber, mirroring task
// mint), then atomically writes the content. An existing slug is an error (not
// ErrNotFound).
func (s *NoteStore) CreateNote(n mtt.Note) (mtt.Note, error) {
	path, err := s.notePath(n.Slug)
	if err != nil {
		return mtt.Note{}, err
	}
	dir := filepath.Join(s.root, dirName, knowledgeDirName)
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
	dir := filepath.Join(s.root, dirName, knowledgeDirName)
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
```

- [ ] **Step 4: Run the tests, verify they pass.**

Run: `go test ./internal/adapter/yaml/ -run TestNoteStore -v`
Expected: PASS (CRUD, not-found/corrupt, traversal rejection).

- [ ] **Step 5: Gate + commit.**

Run: `make check`

```bash
git add internal/adapter/yaml/note.go internal/adapter/yaml/note_test.go
git commit -m "t47: yaml — KnowledgeStore file store (reserve-then-write, slug re-validation)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: `core` — note usecases (`NoteAdder`, `NoteEditor`) + pure list filter

**Files:**
- Create: `internal/core/note.go`
- Test: `internal/core/note_test.go`

**Interfaces:**
- Consumes: `mtt.KnowledgeStore`, `mtt.Note`, `mtt.NoteSlug` (Task 1); the package `canonicalTags`/`anyOrEmptyIntersect` (existing).
- Produces: `core.NewNoteAdder(mtt.KnowledgeStore, func() time.Time) *NoteAdder`, `NoteParams{Slug, Title, Tags, Body}`, `(*NoteAdder).Add(NoteParams) (mtt.Note, error)`; `core.NewNoteEditor(...) *NoteEditor`, `NoteEditParams{Title *string, Tags *[]string, Body *string}`, `(*NoteEditor).Edit(mtt.NoteSlug, NoteEditParams) (mtt.Note, error)`; `core.NoteFilter{Tags []string}`, `core.SelectNotes([]mtt.Note, NoteFilter) []mtt.Note`.

- [ ] **Step 1: Write the failing usecase tests.** Create `internal/core/note_test.go`:

```go
package core

import (
	"errors"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// fakeKB is an in-memory mtt.KnowledgeStore for usecase tests.
type fakeKB struct {
	notes map[mtt.NoteSlug]mtt.Note
}

func newFakeKB() *fakeKB { return &fakeKB{notes: map[mtt.NoteSlug]mtt.Note{}} }

func (f *fakeKB) CreateNote(n mtt.Note) (mtt.Note, error) {
	if _, ok := f.notes[n.Slug]; ok {
		return mtt.Note{}, errors.New("exists")
	}
	f.notes[n.Slug] = n
	return n, nil
}
func (f *fakeKB) GetNote(slug mtt.NoteSlug) (mtt.Note, error) {
	n, ok := f.notes[slug]
	if !ok {
		return mtt.Note{}, mtt.ErrNotFound
	}
	return n, nil
}
func (f *fakeKB) ListNotes() ([]mtt.Note, error) {
	out := make([]mtt.Note, 0, len(f.notes))
	for _, n := range f.notes {
		out = append(out, n)
	}
	return out, nil
}
func (f *fakeKB) UpdateNote(n mtt.Note) (mtt.Note, error) {
	if _, ok := f.notes[n.Slug]; !ok {
		return mtt.Note{}, mtt.ErrNotFound
	}
	f.notes[n.Slug] = n
	return n, nil
}
func (f *fakeKB) DeleteNote(slug mtt.NoteSlug) error {
	if _, ok := f.notes[slug]; !ok {
		return mtt.ErrNotFound
	}
	delete(f.notes, slug)
	return nil
}

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

func TestNoteAdder(t *testing.T) {
	kb := newFakeKB()
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	got, err := NewNoteAdder(kb, fixedClock(ts)).Add(NoteParams{Slug: "auth-design", Title: "Auth", Tags: []string{"Design", "design", "auth"}, Body: "b"})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if !got.Created.Equal(ts) || !got.Updated.Equal(ts) {
		t.Errorf("clock not applied: %+v", got)
	}
	// canonicalTags: deduped + sorted + lowercased.
	if len(got.Tags) != 2 || got.Tags[0] != "auth" || got.Tags[1] != "design" {
		t.Errorf("tags not canonical: %v", got.Tags)
	}
	// Invalid slug rejected.
	if _, err := NewNoteAdder(kb, fixedClock(ts)).Add(NoteParams{Slug: "../x"}); err == nil {
		t.Error("add invalid slug: want error")
	}
}

func TestNoteEditor(t *testing.T) {
	kb := newFakeKB()
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	kb.notes["auth-design"] = mtt.Note{Slug: "auth-design", Title: "Old", Tags: []string{"a"}, Body: "old", Created: created, Updated: created}
	later := created.Add(time.Hour)

	title := "New"
	got, err := NewNoteEditor(kb, fixedClock(later)).Edit("auth-design", NoteEditParams{Title: &title})
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if got.Title != "New" || got.Body != "old" { // only title changed
		t.Errorf("edit applied wrong fields: %+v", got)
	}
	if !got.Created.Equal(created) || !got.Updated.Equal(later) {
		t.Errorf("created must be kept, updated bumped: %+v", got)
	}
	// Tags provided -> whole set replaced.
	tags := []string{"x", "y"}
	got, _ = NewNoteEditor(kb, fixedClock(later)).Edit("auth-design", NoteEditParams{Tags: &tags})
	if len(got.Tags) != 2 || got.Tags[0] != "x" {
		t.Errorf("tags not replaced: %v", got.Tags)
	}
	// Nothing to edit -> error.
	if _, err := NewNoteEditor(kb, fixedClock(later)).Edit("auth-design", NoteEditParams{}); err == nil {
		t.Error("empty edit: want error")
	}
	// Missing note -> ErrNotFound.
	if _, err := NewNoteEditor(kb, fixedClock(later)).Edit("missing", NoteEditParams{Title: &title}); !errors.Is(err, mtt.ErrNotFound) {
		t.Errorf("edit missing: want ErrNotFound, got %v", err)
	}
}

func TestSelectNotes(t *testing.T) {
	older := time.Unix(100, 0).UTC()
	newer := time.Unix(200, 0).UTC()
	notes := []mtt.Note{
		{Slug: "b", Tags: []string{"design"}, Created: older},
		{Slug: "a", Tags: []string{"design"}, Created: newer},
		{Slug: "c", Tags: []string{"ops"}, Created: newer},
	}
	// Empty filter -> all, Created desc then slug asc.
	all := SelectNotes(notes, NoteFilter{})
	if len(all) != 3 || all[0].Slug != "a" || all[1].Slug != "c" || all[2].Slug != "b" {
		t.Fatalf("order: %v", slugs(all))
	}
	// Tag filter -> intersection.
	design := SelectNotes(notes, NoteFilter{Tags: []string{"design"}})
	if len(design) != 2 || design[0].Slug != "a" || design[1].Slug != "b" {
		t.Fatalf("tag filter: %v", slugs(design))
	}
}

func slugs(ns []mtt.Note) []string {
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = string(n.Slug)
	}
	return out
}
```

- [ ] **Step 2: Run it, verify it fails.**

Run: `go test ./internal/core/ -run 'TestNote|TestSelectNotes'`
Expected: FAIL (undefined symbols).

- [ ] **Step 3: Implement `note.go`.** Create `internal/core/note.go`:

```go
package core

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// NoteAdder is the note-create usecase: validate the slug, canonicalize tags, stamp
// Created/Updated from the injected clock, persist via KnowledgeStore.CreateNote.
type NoteAdder struct {
	store mtt.KnowledgeStore
	now   func() time.Time
}

// NewNoteAdder builds a NoteAdder.
func NewNoteAdder(store mtt.KnowledgeStore, now func() time.Time) *NoteAdder {
	return &NoteAdder{store: store, now: now}
}

// NoteParams carries the note-create inputs (already parsed at the CLI boundary).
type NoteParams struct {
	Slug  mtt.NoteSlug
	Title string
	Tags  []string
	Body  string
}

// Add validates and creates the note.
func (a *NoteAdder) Add(p NoteParams) (mtt.Note, error) {
	if !p.Slug.Valid() {
		return mtt.Note{}, fmt.Errorf("invalid note slug %q", string(p.Slug))
	}
	ts := a.now().UTC()
	return a.store.CreateNote(mtt.Note{
		Slug:    p.Slug,
		Title:   p.Title,
		Tags:    canonicalTags(p.Tags),
		Body:    p.Body,
		Created: ts,
		Updated: ts,
	})
}

// NoteEditor is the note-edit usecase: load, apply only the provided fields, bump
// Updated (keep Created), persist via KnowledgeStore.UpdateNote.
type NoteEditor struct {
	store mtt.KnowledgeStore
	now   func() time.Time
}

// NewNoteEditor builds a NoteEditor.
func NewNoteEditor(store mtt.KnowledgeStore, now func() time.Time) *NoteEditor {
	return &NoteEditor{store: store, now: now}
}

// NoteEditParams: a nil pointer means "unchanged". Tags, when non-nil, REPLACES the
// whole set (declarative, not additive).
type NoteEditParams struct {
	Title *string
	Tags  *[]string
	Body  *string
}

// Edit applies the provided fields and persists the note.
func (e *NoteEditor) Edit(slug mtt.NoteSlug, p NoteEditParams) (mtt.Note, error) {
	if !slug.Valid() {
		return mtt.Note{}, fmt.Errorf("invalid note slug %q", string(slug))
	}
	if p.Title == nil && p.Tags == nil && p.Body == nil {
		return mtt.Note{}, errors.New("nothing to edit (provide --title, --tag, --body, or --file)")
	}
	n, err := e.store.GetNote(slug)
	if err != nil {
		return mtt.Note{}, err
	}
	if p.Title != nil {
		n.Title = *p.Title
	}
	if p.Tags != nil {
		n.Tags = canonicalTags(*p.Tags)
	}
	if p.Body != nil {
		n.Body = *p.Body
	}
	n.Updated = e.now().UTC()
	return e.store.UpdateNote(n)
}

// NoteFilter filters a note list. Tags is OR-within (a note matches if it carries any
// filter tag; an empty filter matches all). Filter tags are pre-normalized (CLI toTags).
type NoteFilter struct {
	Tags []string
}

// SelectNotes filters notes and orders them Created desc, tie-broken by slug (opaque
// string) for determinism. Pure — no store, no clock (mirrors core.Select).
func SelectNotes(notes []mtt.Note, f NoteFilter) []mtt.Note {
	out := make([]mtt.Note, 0, len(notes))
	for _, n := range notes {
		if anyOrEmptyIntersect(f.Tags, n.Tags) {
			out = append(out, n)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].Created.Equal(out[j].Created) {
			return out[i].Created.After(out[j].Created)
		}
		return out[i].Slug < out[j].Slug
	})
	return out
}
```

- [ ] **Step 4: Run the tests, verify they pass.**

Run: `go test ./internal/core/ -run 'TestNote|TestSelectNotes' -v`
Expected: PASS.

- [ ] **Step 5: Gate + commit.**

Run: `make check`

```bash
git add internal/core/note.go internal/core/note_test.go
git commit -m "t47: core — note add/edit usecases + pure list filter

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: CLI — `mtt note` command group + JSON + e2e

**Files:**
- Create: `internal/cli/note.go`
- Modify: `internal/cli/errors.go` (add `noteNotFound`)
- Modify: `internal/cli/root.go:48` (register `newNoteCmd()`)
- Test: `internal/cli/testdata/scripts/note.txt`

**Interfaces:**
- Consumes: `core.NewNoteAdder`/`NoteParams`, `core.NewNoteEditor`/`NoteEditParams`, `core.SelectNotes`/`NoteFilter` (Task 4); `yaml.NewKnowledgeStore` (Task 3); `mtt.NewNoteSlug` (Task 1); in-package `oneID`, `toTags`, `jsonFlag`, `writeJSON`, `projectRoot` (existing).
- Produces: `newNoteCmd() *cobra.Command`; `noteJSON`/`toNoteJSON`; `noteNotFound(mtt.NoteSlug) error`.

- [ ] **Step 1: Add `noteNotFound` to `errors.go`.** Append to `internal/cli/errors.go`:

```go
// noteNotFound is the uniform "note not found" error for single-note-by-slug
// commands. Wrapping mtt.ErrNotFound lets exitCode map it to 4 (like taskNotFound).
func noteNotFound(slug mtt.NoteSlug) error {
	return fmt.Errorf("note %q: %w", string(slug), mtt.ErrNotFound)
}
```

- [ ] **Step 2: Implement `note.go` (the whole command group).** Create `internal/cli/note.go`:

```go
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newNoteCmd builds `mtt note` with add/list/show/edit/rm subcommands (the dep pattern).
func newNoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "note",
		Short: "Manage knowledge-base notes",
	}
	cmd.AddCommand(newNoteAddCmd(), newNoteListCmd(), newNoteShowCmd(), newNoteEditCmd(), newNoteRmCmd())
	return cmd
}

// noteJSON is the CLI's machine-readable view of a note: slug always present
// (identity), tags a non-null array ([] when empty).
type noteJSON struct {
	Slug    string   `json:"slug"`
	Title   string   `json:"title,omitempty"`
	Tags    []string `json:"tags"`
	Body    string   `json:"body"`
	Created string   `json:"created"`
	Updated string   `json:"updated"`
}

func toNoteJSON(n mtt.Note) noteJSON {
	tags := n.Tags
	if tags == nil {
		tags = []string{}
	}
	return noteJSON{
		Slug:    string(n.Slug),
		Title:   n.Title,
		Tags:    tags,
		Body:    n.Body,
		Created: n.Created.UTC().Format(time.RFC3339),
		Updated: n.Updated.UTC().Format(time.RFC3339),
	}
}

// readNoteBody resolves the body from the mutually-exclusive --body / --file
// (--file - = stdin). None provided -> "" (empty body allowed).
func readNoteBody(cmd *cobra.Command, body, file string) (string, error) {
	bodySet, fileSet := cmd.Flags().Changed("body"), cmd.Flags().Changed("file")
	if bodySet && fileSet {
		return "", errors.New("provide at most one of --body or --file")
	}
	switch {
	case bodySet:
		return body, nil
	case fileSet:
		if file == "-" {
			data, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return "", fmt.Errorf("read stdin: %w", err)
			}
			return string(data), nil
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", file, err)
		}
		return string(data), nil
	}
	return "", nil
}

func newNoteAddCmd() *cobra.Command {
	var (
		title, body, file string
		tags              []string
	)
	cmd := &cobra.Command{
		Use:   "add <slug>",
		Short: "Create a knowledge note",
		Args:  oneID("provide exactly one slug (example: mtt note add auth-design)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug, err := mtt.NewNoteSlug(args[0])
			if err != nil {
				return err
			}
			normTags, err := toTags(tags)
			if err != nil {
				return err
			}
			b, err := readNoteBody(cmd, body, file)
			if err != nil {
				return err
			}
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			note, err := core.NewNoteAdder(yaml.NewKnowledgeStore(root), time.Now).Add(core.NoteParams{Slug: slug, Title: title, Tags: normTags, Body: b})
			if err != nil {
				return err
			}
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), toNoteJSON(note))
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "created %s\n", note.Slug)
			return err
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "note title")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "add a tag (repeatable)")
	cmd.Flags().StringVar(&body, "body", "", "note body (markdown)")
	cmd.Flags().StringVar(&file, "file", "", "read the body from a file ('-' for stdin)")
	return cmd
}

func newNoteListCmd() *cobra.Command {
	var tags []string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List knowledge notes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			normTags, err := toTags(tags)
			if err != nil {
				return err
			}
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			notes, err := yaml.NewKnowledgeStore(root).ListNotes()
			if err != nil {
				return err
			}
			sel := core.SelectNotes(notes, core.NoteFilter{Tags: normTags})
			if jsonFlag(cmd) {
				out := make([]noteJSON, 0, len(sel))
				for _, n := range sel {
					out = append(out, toNoteJSON(n))
				}
				return writeJSON(cmd.OutOrStdout(), out)
			}
			var b strings.Builder
			for _, n := range sel {
				fmt.Fprintf(&b, "%s\n", noteLine(n))
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), b.String())
			return err
		},
	}
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "filter by tag (repeatable; OR within)")
	return cmd
}

// noteLine is the one-row list formatter: slug, title (or (untitled)), optional tags.
func noteLine(n mtt.Note) string {
	title := n.Title
	if title == "" {
		title = "(untitled)"
	}
	if len(n.Tags) > 0 {
		return fmt.Sprintf("%s  %s  [%s]", n.Slug, title, strings.Join(n.Tags, ", "))
	}
	return fmt.Sprintf("%s  %s", n.Slug, title)
}

func newNoteShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <slug>",
		Short: "Show a knowledge note (frontmatter + body)",
		Args:  oneID("provide exactly one slug (example: mtt note show auth-design)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug, err := mtt.NewNoteSlug(args[0])
			if err != nil {
				return err
			}
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			note, err := yaml.NewKnowledgeStore(root).GetNote(slug)
			if err != nil {
				if errors.Is(err, mtt.ErrNotFound) {
					return noteNotFound(slug)
				}
				return err
			}
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), toNoteJSON(note))
			}
			return writeNote(cmd, note)
		},
	}
}

// writeNote renders a note for humans: a header then the body.
func writeNote(cmd *cobra.Command, n mtt.Note) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", n.Slug)
	if n.Title != "" {
		fmt.Fprintf(&b, "  title:   %s\n", n.Title)
	}
	if len(n.Tags) > 0 {
		fmt.Fprintf(&b, "  tags:    %s\n", strings.Join(n.Tags, ", "))
	}
	fmt.Fprintf(&b, "  created: %s\n", n.Created.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "  updated: %s\n", n.Updated.UTC().Format(time.RFC3339))
	if n.Body != "" {
		fmt.Fprintf(&b, "\n%s", n.Body)
		if !strings.HasSuffix(n.Body, "\n") {
			b.WriteString("\n")
		}
	}
	_, err := fmt.Fprint(cmd.OutOrStdout(), b.String())
	return err
}

func newNoteEditCmd() *cobra.Command {
	var (
		title, body, file string
		tags              []string
	)
	cmd := &cobra.Command{
		Use:   "edit <slug>",
		Short: "Edit a note's title, tags, and/or body",
		Args:  oneID("provide exactly one slug (example: mtt note edit auth-design)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug, err := mtt.NewNoteSlug(args[0])
			if err != nil {
				return err
			}
			var p core.NoteEditParams
			if cmd.Flags().Changed("title") {
				p.Title = &title
			}
			if cmd.Flags().Changed("tag") {
				normTags, err := toTags(tags)
				if err != nil {
					return err
				}
				p.Tags = &normTags
			}
			if cmd.Flags().Changed("body") || cmd.Flags().Changed("file") {
				b, err := readNoteBody(cmd, body, file)
				if err != nil {
					return err
				}
				p.Body = &b
			}
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			note, err := core.NewNoteEditor(yaml.NewKnowledgeStore(root), time.Now).Edit(slug, p)
			if err != nil {
				if errors.Is(err, mtt.ErrNotFound) {
					return noteNotFound(slug)
				}
				return err
			}
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), toNoteJSON(note))
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "updated %s\n", note.Slug)
			return err
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "replace the tag set (repeatable)")
	cmd.Flags().StringVar(&body, "body", "", "new body (markdown)")
	cmd.Flags().StringVar(&file, "file", "", "read the new body from a file ('-' for stdin)")
	return cmd
}

func newNoteRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <slug>",
		Short: "Delete a knowledge note",
		Args:  oneID("provide exactly one slug (example: mtt note rm auth-design)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug, err := mtt.NewNoteSlug(args[0])
			if err != nil {
				return err
			}
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			store := yaml.NewKnowledgeStore(root)
			var note mtt.Note
			if jsonFlag(cmd) { // capture before delete so --json can echo the removed note
				note, err = store.GetNote(slug)
				if err != nil {
					if errors.Is(err, mtt.ErrNotFound) {
						return noteNotFound(slug)
					}
					return err
				}
			}
			if err := store.DeleteNote(slug); err != nil {
				if errors.Is(err, mtt.ErrNotFound) {
					return noteNotFound(slug)
				}
				return err
			}
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), toNoteJSON(note))
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", slug)
			return err
		},
	}
}
```

- [ ] **Step 3: Register the command.** In `internal/cli/root.go`, change line 48 from:

```go
		newUseCmd(), newRmCmd(), newRoadmapCmd(), newTagCmd(), newTagsCmd(), newDoCmd())
```

to:

```go
		newUseCmd(), newRmCmd(), newRoadmapCmd(), newTagCmd(), newTagsCmd(), newDoCmd(),
		newNoteCmd())
```

- [ ] **Step 4: Build to verify it compiles.**

Run: `go build ./...`
Expected: success.

- [ ] **Step 5: Write the e2e script.** Create `internal/cli/testdata/scripts/note.txt`:

```
# note CRUD + slug validation + json + stdin body
exec mtt init

# create with title, tags, inline body
exec mtt note add auth-design --title 'Auth design' --tag design --tag auth --body 'First body.'
stdout 'created auth-design'

# show renders header + body
exec mtt note show auth-design
stdout 'title:   Auth design'
stdout 'First body\.'

# list finds it by tag (slug on its own row)
exec mtt note list --tag design
stdout '^auth-design'

# json: slug present, tags a (non-null) array
exec mtt note show auth-design --json
stdout '"slug": "auth-design"'
stdout '"tags":'

# edit replaces tags + bumps the note; created is kept
exec mtt note edit auth-design --tag design
stdout 'updated auth-design'

# body via stdin (--file -)
stdin body.md
exec mtt note add via-stdin --file -
exec mtt note show via-stdin
stdout 'hello from stdin'

# empty body allowed
exec mtt note add stub --title Stub
stdout 'created stub'

# create refuses an existing slug (no clobber)
! exec mtt note add auth-design --title dup
stderr 'already exists'

# invalid slugs rejected (traversal + uppercase)
! exec mtt note add ../evil
stderr 'invalid note slug'
! exec mtt note add Auth
stderr 'invalid note slug'

# not-found surfaces the note noun (exit 4 via ErrNotFound mapping)
! exec mtt note show missing
stderr 'note "missing"'

# list --json is a non-null array
exec mtt note list --json
stdout '\['

# rm deletes; a second show fails
exec mtt note rm stub
stdout 'removed stub'
! exec mtt note show stub

-- body.md --
hello from stdin
```

- [ ] **Step 6: Run the e2e (and the full CLI suite).**

Run: `go test ./internal/cli/ -run 'TestScript/note'`
Expected: PASS.
Run: `go test ./internal/cli/`
Expected: PASS (no regressions).

- [ ] **Step 7: Gate + commit.**

Run: `make check`

```bash
git add internal/cli/note.go internal/cli/errors.go internal/cli/root.go internal/cli/testdata/scripts/note.txt
git commit -m "t47: cli — mtt note add/list/show/edit/rm (+ --json, e2e)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: Docs sync

**Files:**
- Modify: `docs/architecture/model.go` (Note + KnowledgeStore blocks)
- Modify: `DESIGN.md` + `DESIGN.ru.md` (KB rows: Data layout, KB & refs, Adapter capabilities)
- Modify: `CLI_REFERENCE.md` + `CLI_REFERENCE.ru.md` (the `mtt note` group)
- Modify: `internal/adapter/yaml/CLAUDE.md`, `internal/cli/CLAUDE.md`, `pkg/mtt/CLAUDE.md`

- [ ] **Step 1: Update `docs/architecture/model.go`.** In the `Note` block, set the fields to `Slug/Title/Tags/Body/Created/Updated` and drop `Version/Predecessor` (add a comment "versioning deferred to t6"). In the `KnowledgeStore` block, drop the `version` param on `GetNote`, add `ListNotes/UpdateNote/DeleteNote`, and note `NoteHistory`/versioning + `CapKnowledge`/`Capabilities()` remain deferred (T3).

- [ ] **Step 2: Update `DESIGN.md` and mirror in `DESIGN.ru.md`.** Grep both first: `grep -n 'phase 5\|knowledge\|KnowledgeStore\|KB' DESIGN.md DESIGN.ru.md`. Then: in the "Data layout" block, drop `[phase 5]` from the `knowledge/<slug>.md` line (the base store ships now; search/versioning stay deferred); in the "KB & refs" decision row, note the base `KnowledgeStore` is real in YAML; in "Adapter capabilities", list `KnowledgeStore` as implemented (search + versioning still optional/deferred). Keep EN and RU in sync (the "parallel occurrences" rule).

- [ ] **Step 3: Update `CLI_REFERENCE.md` + `CLI_REFERENCE.ru.md`.** Add a `mtt note` section documenting `add <slug> [--title] [--tag]… [--body|--file(-)]`, `list [--tag]… [--json]`, `show <slug> [--json]`, `edit <slug> [--title|--tag|--body|--file]`, `rm <slug>`, and the exit-4 not-found behavior. EN + RU in sync.

- [ ] **Step 4: Update the package `CLAUDE.md` files.**
  - `pkg/mtt/CLAUDE.md`: under "Named identities", record the **`NoteSlug` structural-validation carve-out** (it parses structure — a kebab-ASCII path segment — unlike the opaque `TaskID`/`TypeName`/`StatusName`); mention `Note` + the `KnowledgeStore` port (no versioning/search in the base).
  - `internal/adapter/yaml/CLAUDE.md`: add `NewKnowledgeStore`/`NoteStore` — one `.mtt/knowledge/<slug>.md` per note, the **serialization contract** (frontmatter + verbatim body; never whole-file unmarshal), reserve-then-write no-clobber, slug re-validation at every path-building method + on load.
  - `internal/cli/CLAUDE.md`: add the `mtt note` group (add/list/show/edit/rm; body via `--body`/`--file`/`--file -`; `noteJSON`; `noteNotFound` → exit 4).

- [ ] **Step 5: Gate + commit.**

Run: `make check`
Expected: `OK` (docs-only, but confirm nothing references a moved symbol).

```bash
git add docs/architecture/model.go DESIGN.md DESIGN.ru.md CLI_REFERENCE.md CLI_REFERENCE.ru.md pkg/mtt/CLAUDE.md internal/adapter/yaml/CLAUDE.md internal/cli/CLAUDE.md
git commit -m "t47: docs — KB seed (model.go, DESIGN±ru, CLI_REFERENCE±ru, CLAUDE.md)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Final acceptance (map to the spec's ACs)

- [ ] **AC-1** (CRUD loop) → Task 5 e2e `note.txt`.
- [ ] **AC-2** (reviewable file; goldens incl. body-with-`---` + trailing-newline) → Task 2 (`TestNoteRoundTrip`, `TestNoteGolden`).
- [ ] **AC-3** (slug validation, defense-in-depth incl. traversal) → Task 1 (`TestNewNoteSlug`), Task 3 (`TestNoteStoreRejectsTraversalSlug`), Task 5 e2e.
- [ ] **AC-4** (not-found → exit 4 via `noteNotFound`) → Task 3 (`TestNoteStoreNotFoundAndCorrupt`), Task 5 (`noteNotFound` + e2e wording; exit 4 inherited from the existing `errors.Is(ErrNotFound)` mapping — no `exitCode` change).
- [ ] **AC-5** (reserve-then-write, no clobber) → Task 3 (`TestNoteStoreCRUD` clobber assertion).
- [ ] **AC-6** (tags normalized/sorted, no text extraction) → Task 4 (`TestNoteAdder`).
- [ ] **AC-7** (body via `--body`/`--file`/`--file -`) → Task 5 (`readNoteBody` + e2e stdin).
- [ ] **AC-8** (`--json` shape: slug always, tags non-null, list `[]`) → Task 5 (`toNoteJSON` + e2e).
- [ ] **AC-9** (`make check` green; docs synced) → every task's gate + Task 6.

## Self-review notes

- **Spec coverage:** D1 → Task 1; D2 (Note, tags-no-extraction) → Tasks 1+4; D3 (port, no CapKnowledge, ErrNotFound) → Tasks 1+5; D4 (serialization, reserve-then-write) → Tasks 2+3; D5 (core usecases, filter) → Task 4; D6 (CLI, JSON) → Task 5; docs-sync → Task 6. No spec section is unmapped.
- **Type consistency:** `NoteParams{Slug,Title,Tags,Body}`, `NoteEditParams{Title,Tags,Body}` (pointers), `NoteFilter{Tags}`, `toNoteJSON`/`noteJSON`, `NewKnowledgeStore`/`NoteStore`, `marshalNote`/`parseNote`, `noteNotFound` — names match across the tasks that define and consume them.
- **No placeholders:** every code/test/e2e block is complete; goldens are generated via `-update` in Task 2 (the repo's convention) then committed.
- **Out of scope (unchanged):** refs (t1), search + versioning (t6), `CapKnowledge`/`mtt caps` (e4_t6), mass knowledge migration (post-seed).
