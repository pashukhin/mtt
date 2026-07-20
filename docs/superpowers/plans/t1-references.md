# references (t1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire and verify structured references (`Ref{Kind,ID,Label}`) on tasks and notes — `mtt ref` / `mtt note ref` groups, capability-aware verification, computed cross-store backlinks, a `mtt check` integrity sweep, and a refuse-by-default deletion guard.

**Architecture:** Hexagonal, unchanged. The `Ref` types already exist in `pkg/mtt`; the YAML task adapter already round-trips `Task.Refs`/`Comment.Refs`. This plan adds `Note.Refs` (domain + frontmatter), pure `core` logic (ref-set algebra, verification → `RefStatus`, a computed `Backlinks` value, a `check` sweep, ref editors, a `NoteRemover`, an extended task `Remover`), and thin CLI wiring. No new ports (`TaskStore`/`KnowledgeStore`/`AuditStore` all exist). `RefStatus`/`Backlinks` are `core`-derived, never in the `pkg/mtt` contract and never stored (like `Index`/`DepGraph`).

**Tech Stack:** Go 1.23+, cobra CLI, `gopkg.in/yaml.v3`, `testscript` (txtar e2e), table-driven unit tests, golden files.

## Global Constraints

- **Spec of record:** `docs/superpowers/specs/t1-references.md`. Every decision (D1–D11) there is binding.
- **TDD:** red → green → refactor. Write the failing test first, watch it fail, then implement. `make check` (gofmt + vet + golangci-lint v2 + `go test -race -cover` + build) green before every commit.
- **Layering:** `core` imports **no** `adapter/*`; `pkg/mtt` domain carries **no** yaml/json tags; storage only through ports; **no type/status-name literals** in code (`RefKind` is a closed domain vocabulary — that is allowed).
- **Kinds:** live kinds are `note`/`task`/`url`. `comment` is **rejected at the CLI input boundary** (exit 1, "comments arrive in t2"); the `RefComment` constant and `RefKind.Valid()` stay unchanged.
- **Ref identity** = the natural key `(kind, target)` (`target` = `Ref.ID`, a plain `string`). Stored refs are **deduped by key and sorted by `(kind, target)`**. `--label` is annotation, not identity.
- **Write policy = warn-not-block:** a dangling/unverified target is **stored** with a stderr warning and **exit 0**. The only hard write failure is malformed input (bad `kind:target`, `comment:`, malformed URL/id/slug) → exit 1. Carrier-not-found → exit 4.
- **`ref rm` / `note ref rm` of an absent key = idempotent no-op (exit 0)** (like `dep rm`/`tag rm`).
- **Deletion guard = refuse-by-default + `--force`** (irreversibility); `--force` forces `--who`/`--why` + writes an audit record before deleting.
- **Exit codes:** `1` usage · `2` attribution (`rm --force`/`note rm --force`) · `4` carrier not found · `7` `mtt check` found ≥1 dangling. (`3`/`5`/`6` unchanged, not used here.)
- **`refs`/`backlinks` are `show`-scoped in JSON:** they appear only in `show`/`note show` and `ref list`/`note ref list`; `taskJSON`/`noteJSON` used by `add`/`list`/`edit` stay lean (no refs field).
- **No network in tests.** URLs are validated syntactically only (`unverified`, never resolved).
- **Docs are bilingual** where applicable (EN + RU): `DESIGN`, `CLI_REFERENCE`. Grep all parallel occurrences before editing.

---

## File structure

**Create:**
- `internal/core/ref.go` — pure ref-set algebra + `RefStatus` + `VerifyRef`.
- `internal/core/backlinks.go` — `RefKey`/`Referent`/`Backlinks`/`NewBacklinks` + `CheckRefs` + `ErrDanglingRefs`.
- `internal/core/refedit.go` — `RefEditor` (task) + `NoteRefEditor` (note) mutations.
- `internal/core/noteremove.go` — `NoteRemover` (guard + `--force` + audit).
- `internal/cli/ref.go` — `parseRefArg`, shared render/JSON helpers, `mtt ref add/rm/list`.
- `internal/cli/noteref.go` — `mtt note ref add/rm/list`.
- `internal/cli/check.go` — `mtt check`.
- Test files alongside each (`*_test.go`) + golden `internal/adapter/yaml/testdata/note_refs.md`.

**Modify:**
- `pkg/mtt/note.go` — add `Refs []Ref` to `Note`.
- `internal/adapter/yaml/note_dto.go` — `ymlNote.Refs` + map in `marshalNote`/`parseNote`.
- `internal/core/add.go` — `AddParams.Refs`; `internal/core/note.go` — `NoteParams.Refs`.
- `internal/core/remove.go` — extend `externalReferencingIDs`, `Remove`/`RemoveMany` to take `Backlinks`.
- `internal/cli/add.go`, `internal/cli/note.go` — `--ref` flag; wire `note ref` group + `note rm --force`/guard.
- `internal/cli/rm.go` — build+pass `Backlinks`.
- `internal/cli/show.go`, `internal/cli/json.go` — Refs/Backlinks sections + JSON.
- `internal/cli/root.go` — register `newRefCmd()`, `newCheckCmd()`; map `ErrDanglingRefs`→7.
- Docs + `CLAUDE.md` files (Task 12).

---

## Task 1: `Note.Refs` domain field + YAML frontmatter round-trip

**Files:**
- Modify: `pkg/mtt/note.go` (add `Refs []Ref` to `Note`)
- Modify: `internal/adapter/yaml/note_dto.go` (`ymlNote.Refs`, `marshalNote`, `parseNote`)
- Create: `internal/adapter/yaml/testdata/golden/note_refs.md` (goldens live under `testdata/golden/`)
- Test: `internal/adapter/yaml/note_dto_test.go` (extend; add a `TestNoteGolden` table row)

**Interfaces:**
- Consumes: existing `ymlRef`, `fromDomainRefs(rs []mtt.Ref) []ymlRef`, `toDomainRefs(rs []ymlRef) []mtt.Ref` from `internal/adapter/yaml/task_dto.go` (same package — reuse, do NOT define a second ref DTO).
- Produces: `mtt.Note.Refs []mtt.Ref`; frontmatter `refs:` block after `tags`, before `created`.

- [ ] **Step 1: Write the failing test** — append to `internal/adapter/yaml/note_dto_test.go`:

```go
func TestMarshalParseNoteWithRefs(t *testing.T) {
	created, _ := time.Parse(time.RFC3339, "2026-07-20T10:00:00Z")
	n := mtt.Note{
		Slug: mtt.NoteSlug("auth-design"), Title: "Auth", Tags: []string{"design"},
		Refs: []mtt.Ref{
			{Kind: mtt.RefTask, ID: "t2"},
			{Kind: mtt.RefURL, ID: "https://example.com/x", Label: "ext"},
		},
		Body: "# Auth\n", Created: created, Updated: created,
	}
	data, err := marshalNote(n)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := parseNote(n.Slug, data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !reflect.DeepEqual(got.Refs, n.Refs) {
		t.Fatalf("refs round-trip: got %+v want %+v", got.Refs, n.Refs)
	}
	if got.Body != n.Body {
		t.Fatalf("body: got %q want %q", got.Body, n.Body)
	}
}
```

Also add a byte-identity guard that a **refs-free** note is unchanged (append):

```go
func TestMarshalNoteNoRefsUnchanged(t *testing.T) {
	created, _ := time.Parse(time.RFC3339, "2026-07-20T10:00:00Z")
	n := mtt.Note{Slug: "m", Created: created, Updated: created}
	data, err := marshalNote(n)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "refs:") {
		t.Fatalf("empty refs must be omitted, got:\n%s", data)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/adapter/yaml/ -run 'NoteWithRefs|NoRefsUnchanged'`
Expected: FAIL — `mtt.Note` has no field `Refs` (compile error).

- [ ] **Step 3: Add the domain field** — in `pkg/mtt/note.go`, add `Refs` to the `Note` struct after `Body` (domain field order is cosmetic — no tags):

```go
// Note is a knowledge-base entry. Refs are verifiable references (t1); like a
// task's, they are informational and non-blocking.
type Note struct {
	Slug    NoteSlug
	Title   string
	Tags    []string
	Body    string
	Refs    []Ref
	Created time.Time
	Updated time.Time
}
```

- [ ] **Step 4: Serialize refs in the frontmatter** — in `internal/adapter/yaml/note_dto.go`, add `Refs` to `ymlNote` (after `Tags`, before `Created`) and map both ways:

```go
type ymlNote struct {
	Title   string   `yaml:"title,omitempty"`
	Tags    []string `yaml:"tags,omitempty"`
	Refs    []ymlRef `yaml:"refs,omitempty"`
	Created string   `yaml:"created"`
	Updated string   `yaml:"updated"`
}
```

In `marshalNote`, set `Refs: fromDomainRefs(n.Refs)` in the `ymlNote{...}` literal. In `parseNote`, set `Refs: toDomainRefs(yn.Refs)` in the returned `mtt.Note{...}` literal.

- [ ] **Step 5: Create the golden** `internal/adapter/yaml/testdata/golden/note_refs.md` and add a `{"refs", <note-with-refs>, "note_refs.md"}` row to the existing `TestNoteGolden` table (it does `filepath.Join("testdata","golden",tc.file)`). The package has an `-update` flag (see `init_test.go`) — run `go test ./internal/adapter/yaml/ -run TestNoteGolden -update` to generate the authoritative bytes, then inspect the diff. Expected shape (note: yaml.v3 **quotes** RFC3339 timestamps, as `note_full.md` shows):

```markdown
---
title: Auth
tags:
    - design
refs:
    - kind: task
      id: t2
    - kind: url
      id: https://example.com/x
      label: ext
created: "2026-07-20T10:00:00Z"
updated: "2026-07-20T10:00:00Z"
---
# Auth
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/adapter/yaml/ ./pkg/mtt/`
Expected: PASS (including the pre-existing `note_min.md`/`note_full.md` goldens — byte-identical).

- [ ] **Step 7: Commit**

```bash
git add pkg/mtt/note.go internal/adapter/yaml/note_dto.go internal/adapter/yaml/note_dto_test.go internal/adapter/yaml/testdata/golden/note_refs.md
git commit -m "t1: Note.Refs domain field + frontmatter round-trip"
```

---

## Task 2: Core — ref-set algebra + `RefStatus` + verification

**Files:**
- Create: `internal/core/ref.go`
- Test: `internal/core/ref_test.go`

**Interfaces:**
- Produces:
  - `type RefStatus string` with `RefOK RefStatus = "ok"`, `RefDangling RefStatus = "dangling"`, `RefUnverified RefStatus = "unverified"`.
  - `func canonicalRefs(refs []mtt.Ref) []mtt.Ref` — dedup by `(Kind,ID)` keeping the **last** occurrence (so upsert's appended value wins), sorted by `(Kind,ID)`.
  - `func upsertRef(refs []mtt.Ref, r mtt.Ref, setLabel bool) []mtt.Ref` — if `(r.Kind,r.ID)` present: overwrite its label only when `setLabel`; else append `r`. Returns `canonicalRefs`.
  - `func removeRef(refs []mtt.Ref, kind mtt.RefKind, id string) ([]mtt.Ref, bool)` — drop the `(kind,id)` entry; `bool` = found.
  - `func VerifyRef(r mtt.Ref, taskExists func(mtt.TaskID) bool, noteExists func(mtt.NoteSlug) bool) RefStatus` — capability-aware; `noteExists == nil` ⇒ note is `RefUnverified`.

- [ ] **Step 1: Write the failing test** — `internal/core/ref_test.go`:

```go
package core

import (
	"reflect"
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestCanonicalRefsDedupSort(t *testing.T) {
	in := []mtt.Ref{
		{Kind: mtt.RefTask, ID: "t2"},
		{Kind: mtt.RefNote, ID: "a"},
		{Kind: mtt.RefTask, ID: "t2", Label: "new"}, // dup key, last label wins
	}
	got := canonicalRefs(in)
	want := []mtt.Ref{{Kind: mtt.RefNote, ID: "a"}, {Kind: mtt.RefTask, ID: "t2", Label: "new"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestUpsertRef(t *testing.T) {
	refs := []mtt.Ref{{Kind: mtt.RefTask, ID: "t2", Label: "old"}}
	// re-add with label -> overwrite
	got := upsertRef(refs, mtt.Ref{Kind: mtt.RefTask, ID: "t2", Label: "new"}, true)
	if got[0].Label != "new" {
		t.Fatalf("label overwrite: %+v", got)
	}
	// re-add without label -> unchanged (idempotent)
	got = upsertRef(refs, mtt.Ref{Kind: mtt.RefTask, ID: "t2"}, false)
	if got[0].Label != "old" {
		t.Fatalf("no-label re-add must not clear: %+v", got)
	}
	// new key -> appended
	got = upsertRef(refs, mtt.Ref{Kind: mtt.RefURL, ID: "https://x/"}, false)
	if len(got) != 2 {
		t.Fatalf("append: %+v", got)
	}
}

func TestRemoveRef(t *testing.T) {
	refs := []mtt.Ref{{Kind: mtt.RefTask, ID: "t2"}, {Kind: mtt.RefNote, ID: "a"}}
	got, found := removeRef(refs, mtt.RefTask, "t2")
	if !found || len(got) != 1 || got[0].Kind != mtt.RefNote {
		t.Fatalf("remove: %+v found=%v", got, found)
	}
	if _, found := removeRef(refs, mtt.RefTask, "nope"); found {
		t.Fatal("absent key must report not-found")
	}
}

func TestVerifyRef(t *testing.T) {
	taskExists := func(id mtt.TaskID) bool { return id == "t2" }
	noteExists := func(s mtt.NoteSlug) bool { return s == "a" }
	cases := []struct {
		r    mtt.Ref
		ne   func(mtt.NoteSlug) bool
		want RefStatus
	}{
		{mtt.Ref{Kind: mtt.RefTask, ID: "t2"}, noteExists, RefOK},
		{mtt.Ref{Kind: mtt.RefTask, ID: "t9"}, noteExists, RefDangling},
		{mtt.Ref{Kind: mtt.RefNote, ID: "a"}, noteExists, RefOK},
		{mtt.Ref{Kind: mtt.RefNote, ID: "z"}, noteExists, RefDangling},
		{mtt.Ref{Kind: mtt.RefNote, ID: "a"}, nil, RefUnverified}, // no KB wired
		{mtt.Ref{Kind: mtt.RefURL, ID: "https://x/"}, noteExists, RefUnverified},
	}
	for _, c := range cases {
		if got := VerifyRef(c.r, taskExists, c.ne); got != c.want {
			t.Fatalf("%+v: got %q want %q", c.r, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/core/ -run 'CanonicalRefs|UpsertRef|RemoveRef|VerifyRef'`
Expected: FAIL — undefined symbols.

- [ ] **Step 3: Implement** `internal/core/ref.go`:

```go
package core

import (
	"sort"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// RefStatus is the derived resolution state of a reference (never stored).
type RefStatus string

const (
	RefOK         RefStatus = "ok"
	RefDangling   RefStatus = "dangling"
	RefUnverified RefStatus = "unverified"
)

// canonicalRefs returns refs deduped by (Kind,ID) — the natural key — keeping the
// LAST occurrence (so an upsert's appended value wins), sorted by (Kind,ID).
func canonicalRefs(refs []mtt.Ref) []mtt.Ref {
	last := make(map[[2]string]mtt.Ref, len(refs))
	order := make([][2]string, 0, len(refs))
	for _, r := range refs {
		k := [2]string{string(r.Kind), r.ID}
		if _, seen := last[k]; !seen {
			order = append(order, k)
		}
		last[k] = r
	}
	out := make([]mtt.Ref, 0, len(order))
	for _, k := range order {
		out = append(out, last[k])
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// upsertRef adds r to refs by its natural key. An existing key keeps its label
// unless setLabel is true (then r.Label overwrites); a new key is appended. The
// result is canonicalized.
func upsertRef(refs []mtt.Ref, r mtt.Ref, setLabel bool) []mtt.Ref {
	out := make([]mtt.Ref, 0, len(refs)+1)
	found := false
	for _, e := range refs {
		if e.Kind == r.Kind && e.ID == r.ID {
			found = true
			if setLabel {
				e.Label = r.Label
			}
		}
		out = append(out, e)
	}
	if !found {
		out = append(out, r)
	}
	return canonicalRefs(out)
}

// removeRef drops the (kind,id) entry; the bool reports whether it was present.
func removeRef(refs []mtt.Ref, kind mtt.RefKind, id string) ([]mtt.Ref, bool) {
	out := make([]mtt.Ref, 0, len(refs))
	found := false
	for _, e := range refs {
		if e.Kind == kind && e.ID == id {
			found = true
			continue
		}
		out = append(out, e)
	}
	return out, found
}

// VerifyRef resolves a ref's status capability-aware. taskExists is always
// available; noteExists is nil when no KnowledgeStore is wired (then a note ref is
// unverified). url is external — always unverified. Any other kind (e.g. an
// unreachable comment) is unverified.
func VerifyRef(r mtt.Ref, taskExists func(mtt.TaskID) bool, noteExists func(mtt.NoteSlug) bool) RefStatus {
	switch r.Kind {
	case mtt.RefTask:
		if taskExists(mtt.TaskID(r.ID)) {
			return RefOK
		}
		return RefDangling
	case mtt.RefNote:
		if noteExists == nil {
			return RefUnverified
		}
		if noteExists(mtt.NoteSlug(r.ID)) {
			return RefOK
		}
		return RefDangling
	default: // url, and any not-yet-supported kind
		return RefUnverified
	}
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/core/ -run 'CanonicalRefs|UpsertRef|RemoveRef|VerifyRef'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/ref.go internal/core/ref_test.go
git commit -m "t1: core ref-set algebra + RefStatus + capability-aware verification"
```

---

## Task 3: Core — `Backlinks` + `CheckRefs`

**Files:**
- Create: `internal/core/backlinks.go`
- Test: `internal/core/backlinks_test.go`

**Interfaces:**
- Consumes: `RefStatus`, `VerifyRef` (Task 2).
- Produces:
  - `type RefKey struct { Kind mtt.RefKind; Target string }`
  - `type Referent struct { Carrier mtt.RefKind; ID string; Label string }` (`Carrier` ∈ `RefTask`/`RefNote`)
  - `type Backlinks map[RefKey][]Referent`
  - `func NewBacklinks(tasks []mtt.Task, notes []mtt.Note) Backlinks`
  - `func (b Backlinks) To(kind mtt.RefKind, target string) []Referent`
  - `type CheckFinding struct { CarrierKind mtt.RefKind; CarrierID string; Ref mtt.Ref; Status RefStatus }`
  - `func CheckRefs(tasks []mtt.Task, notes []mtt.Note, kbWired bool) []CheckFinding` — all non-`ok` findings, deterministic order.
  - `var ErrDanglingRefs = errors.New("mtt: dangling references")`

- [ ] **Step 1: Write the failing test** — `internal/core/backlinks_test.go`:

```go
package core

import (
	"reflect"
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func fixtures() ([]mtt.Task, []mtt.Note) {
	tasks := []mtt.Task{
		{ID: "t1"},
		{ID: "t2", Refs: []mtt.Ref{{Kind: mtt.RefTask, ID: "t1", Label: "blocks"}, {Kind: mtt.RefTask, ID: "t9"}}},
	}
	notes := []mtt.Note{
		{Slug: "a", Refs: []mtt.Ref{{Kind: mtt.RefTask, ID: "t1"}, {Kind: mtt.RefURL, ID: "https://x/"}}},
	}
	return tasks, notes
}

func TestBacklinksTo(t *testing.T) {
	tasks, notes := fixtures()
	bl := NewBacklinks(tasks, notes)
	got := bl.To(mtt.RefTask, "t1")
	want := []Referent{{Carrier: mtt.RefNote, ID: "a"}, {Carrier: mtt.RefTask, ID: "t2", Label: "blocks"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("backlinks to t1: got %+v want %+v", got, want)
	}
	if len(bl.To(mtt.RefTask, "t9")) != 1 {
		t.Fatal("t9 should have one backlink (from t2)")
	}
}

func TestCheckRefs(t *testing.T) {
	tasks, notes := fixtures()
	got := CheckRefs(tasks, notes, true)
	// dangling: t2->t9 ; unverified: a->url
	var dangling, unverified int
	for _, f := range got {
		switch f.Status {
		case RefDangling:
			dangling++
		case RefUnverified:
			unverified++
		default:
			t.Fatalf("ok refs must not appear: %+v", f)
		}
	}
	if dangling != 1 || unverified != 1 {
		t.Fatalf("got dangling=%d unverified=%d in %+v", dangling, unverified, got)
	}
}

func TestCheckRefsNoKB(t *testing.T) {
	tasks := []mtt.Task{{ID: "t1", Refs: []mtt.Ref{{Kind: mtt.RefNote, ID: "a"}}}}
	got := CheckRefs(tasks, nil, false) // kb not wired
	if len(got) != 1 || got[0].Status != RefUnverified {
		t.Fatalf("note ref with no KB must be unverified: %+v", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/core/ -run 'Backlinks|CheckRefs'`
Expected: FAIL — undefined symbols.

- [ ] **Step 3: Implement** `internal/core/backlinks.go`:

```go
package core

import (
	"errors"
	"sort"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// ErrDanglingRefs is returned by the CLI check command when the sweep finds >=1
// dangling reference (mapped to exit 7).
var ErrDanglingRefs = errors.New("mtt: dangling references")

// RefKey is a reference's natural key (its target).
type RefKey struct {
	Kind   mtt.RefKind
	Target string
}

// Referent is one carrier that points at a target (a computed backlink entry).
type Referent struct {
	Carrier mtt.RefKind // RefTask or RefNote
	ID      string      // task id or note slug
	Label   string      // the forward ref's own label
}

// Backlinks is the computed inverse index target->referents (never stored).
type Backlinks map[RefKey][]Referent

// NewBacklinks builds the inverse index from a task+note snapshot. Referents are
// sorted (carrier kind, then id) for determinism.
func NewBacklinks(tasks []mtt.Task, notes []mtt.Note) Backlinks {
	b := Backlinks{}
	add := func(carrier mtt.RefKind, id string, refs []mtt.Ref) {
		for _, r := range refs {
			k := RefKey{Kind: r.Kind, Target: r.ID}
			b[k] = append(b[k], Referent{Carrier: carrier, ID: id, Label: r.Label})
		}
	}
	for _, t := range tasks {
		add(mtt.RefTask, string(t.ID), t.Refs)
	}
	for _, n := range notes {
		add(mtt.RefNote, string(n.Slug), n.Refs)
	}
	for k := range b {
		refs := b[k]
		sort.SliceStable(refs, func(i, j int) bool {
			if refs[i].Carrier != refs[j].Carrier {
				return refs[i].Carrier < refs[j].Carrier
			}
			return refs[i].ID < refs[j].ID
		})
	}
	return b
}

// To returns the referents pointing at (kind,target), or nil.
func (b Backlinks) To(kind mtt.RefKind, target string) []Referent {
	return b[RefKey{Kind: kind, Target: target}]
}

// CheckFinding is one non-ok ref discovered by the sweep.
type CheckFinding struct {
	CarrierKind mtt.RefKind
	CarrierID   string
	Ref         mtt.Ref
	Status      RefStatus
}

// CheckRefs sweeps every carrier's refs and returns the non-ok findings (dangling
// and unverified), in a deterministic order (carrier kind, carrier id, then
// (ref kind, ref id)). kbWired controls note verifiability (D5); the existence sets
// are built from the same snapshot.
func CheckRefs(tasks []mtt.Task, notes []mtt.Note, kbWired bool) []CheckFinding {
	taskSet := make(map[mtt.TaskID]bool, len(tasks))
	for _, t := range tasks {
		taskSet[t.ID] = true
	}
	noteSet := make(map[mtt.NoteSlug]bool, len(notes))
	for _, n := range notes {
		noteSet[n.Slug] = true
	}
	taskExists := func(id mtt.TaskID) bool { return taskSet[id] }
	var noteExists func(mtt.NoteSlug) bool
	if kbWired {
		noteExists = func(s mtt.NoteSlug) bool { return noteSet[s] }
	}
	var out []CheckFinding
	sweep := func(ck mtt.RefKind, id string, refs []mtt.Ref) {
		for _, r := range refs {
			st := VerifyRef(r, taskExists, noteExists)
			if st == RefOK {
				continue
			}
			out = append(out, CheckFinding{CarrierKind: ck, CarrierID: id, Ref: r, Status: st})
		}
	}
	for _, t := range tasks {
		sweep(mtt.RefTask, string(t.ID), t.Refs)
	}
	for _, n := range notes {
		sweep(mtt.RefNote, string(n.Slug), n.Refs)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.CarrierKind != b.CarrierKind {
			return a.CarrierKind < b.CarrierKind
		}
		if a.CarrierID != b.CarrierID {
			return a.CarrierID < b.CarrierID
		}
		if a.Ref.Kind != b.Ref.Kind {
			return a.Ref.Kind < b.Ref.Kind
		}
		return a.Ref.ID < b.Ref.ID
	})
	return out
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/core/ -run 'Backlinks|CheckRefs'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/backlinks.go internal/core/backlinks_test.go
git commit -m "t1: core cross-store Backlinks index + CheckRefs sweep"
```

---

## Task 4: Core — `RefEditor` (task) + `NoteRefEditor` (note)

**Files:**
- Create: `internal/core/refedit.go`
- Test: `internal/core/refedit_test.go`

**Interfaces:**
- Consumes: `upsertRef`/`removeRef` (Task 2); the existing test fakes for `mtt.TaskStore`/`mtt.KnowledgeStore` in `internal/core` (reuse whatever the existing `*_test.go` use — search for a fake store in `add_test.go`/`note_test.go`).
- Produces:
  - `type RefEditor struct{...}`; `func NewRefEditor(store mtt.TaskStore, now func() time.Time) *RefEditor`
    - `func (e *RefEditor) AddRef(id mtt.TaskID, r mtt.Ref, setLabel bool) (mtt.Task, error)`
    - `func (e *RefEditor) RemoveRef(id mtt.TaskID, kind mtt.RefKind, target string) (mtt.Task, error)`
  - `type NoteRefEditor struct{...}`; `func NewNoteRefEditor(store mtt.KnowledgeStore, now func() time.Time) *NoteRefEditor`
    - `func (e *NoteRefEditor) AddRef(slug mtt.NoteSlug, r mtt.Ref, setLabel bool) (mtt.Note, error)`
    - `func (e *NoteRefEditor) RemoveRef(slug mtt.NoteSlug, kind mtt.RefKind, target string) (mtt.Note, error)`
  - Both mutate the carrier's `Refs` (canonicalized), bump `Updated`, persist; a not-found carrier wraps `mtt.ErrNotFound` (`task %q`/`note %q`); an absent-key remove is an idempotent no-op (no write, no timestamp bump).

- [ ] **Step 1: Write the failing test** — `internal/core/refedit_test.go` (use the package's existing in-memory store fake; the sketch below assumes `newMemStore()`/`newFakeKB()` helpers — reuse the actual names found in the package's tests):

```go
package core

import (
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)


func TestRefEditorAddRemove(t *testing.T) {
	store := newMemStore(mtt.Task{ID: "t1", Updated: time.Unix(0, 0)}) // reuse the package fake
	e := NewRefEditor(store, testClock)
	got, err := e.AddRef("t1", mtt.Ref{Kind: mtt.RefTask, ID: "t2", Label: "blocks"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Refs) != 1 || got.Refs[0].ID != "t2" || !got.Updated.Equal(testClock()) {
		t.Fatalf("add: %+v", got)
	}
	got, err = e.RemoveRef("t1", mtt.RefTask, "t2")
	if err != nil || len(got.Refs) != 0 {
		t.Fatalf("remove: %+v err=%v", got, err)
	}
	// idempotent absent-key remove: no error
	if _, err := e.RemoveRef("t1", mtt.RefTask, "gone"); err != nil {
		t.Fatalf("absent remove must be no-op: %v", err)
	}
	// carrier not found -> ErrNotFound
	if _, err := e.AddRef("t9", mtt.Ref{Kind: mtt.RefTask, ID: "t2"}, false); err == nil {
		t.Fatal("missing carrier must error")
	}
}
```

(Write the analogous `TestNoteRefEditorAddRemove` over `newFakeKB()` seeded via `kb.CreateNote(mtt.Note{Slug: "a"})`.)

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/core/ -run 'RefEditor'`
Expected: FAIL — undefined symbols.

- [ ] **Step 3: Implement** `internal/core/refedit.go`:

```go
package core

import (
	"errors"
	"fmt"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// RefEditor mutates a task's Refs (informational, non-blocking) and persists via
// TaskStore.Update. No new port — refs ride the Task field (GAP #1, like DependsOn).
type RefEditor struct {
	store mtt.TaskStore
	now   func() time.Time
}

// NewRefEditor wires the usecase.
func NewRefEditor(store mtt.TaskStore, now func() time.Time) *RefEditor {
	return &RefEditor{store: store, now: now}
}

// AddRef upserts r on task id by its natural key (setLabel = --label was given),
// bumps Updated, persists.
func (e *RefEditor) AddRef(id mtt.TaskID, r mtt.Ref, setLabel bool) (mtt.Task, error) {
	t, err := e.load(id)
	if err != nil {
		return mtt.Task{}, err
	}
	t.Refs = upsertRef(t.Refs, r, setLabel)
	t.Updated = e.now().UTC().Truncate(time.Second)
	return e.store.Update(t)
}

// RemoveRef drops the (kind,target) ref from task id; an absent key is an
// idempotent no-op (no write).
func (e *RefEditor) RemoveRef(id mtt.TaskID, kind mtt.RefKind, target string) (mtt.Task, error) {
	t, err := e.load(id)
	if err != nil {
		return mtt.Task{}, err
	}
	refs, found := removeRef(t.Refs, kind, target)
	if !found {
		return t, nil
	}
	t.Refs = refs
	t.Updated = e.now().UTC().Truncate(time.Second)
	return e.store.Update(t)
}

func (e *RefEditor) load(id mtt.TaskID) (mtt.Task, error) {
	t, err := e.store.Get(id)
	if err != nil {
		if errors.Is(err, mtt.ErrNotFound) {
			return mtt.Task{}, fmt.Errorf("task %q: %w", id, mtt.ErrNotFound)
		}
		return mtt.Task{}, fmt.Errorf("load task %q: %w", id, err)
	}
	return t, nil
}

// NoteRefEditor is the note analogue over KnowledgeStore.
type NoteRefEditor struct {
	store mtt.KnowledgeStore
	now   func() time.Time
}

// NewNoteRefEditor wires the usecase.
func NewNoteRefEditor(store mtt.KnowledgeStore, now func() time.Time) *NoteRefEditor {
	return &NoteRefEditor{store: store, now: now}
}

// AddRef upserts r on note slug, bumps Updated, persists.
func (e *NoteRefEditor) AddRef(slug mtt.NoteSlug, r mtt.Ref, setLabel bool) (mtt.Note, error) {
	n, err := e.store.GetNote(slug)
	if err != nil {
		return mtt.Note{}, err // GetNote already returns bare ErrNotFound; CLI wraps to noteNotFound
	}
	n.Refs = upsertRef(n.Refs, r, setLabel)
	n.Updated = e.now().UTC()
	return e.store.UpdateNote(n)
}

// RemoveRef drops the (kind,target) ref from note slug; absent key = idempotent no-op.
func (e *NoteRefEditor) RemoveRef(slug mtt.NoteSlug, kind mtt.RefKind, target string) (mtt.Note, error) {
	n, err := e.store.GetNote(slug)
	if err != nil {
		return mtt.Note{}, err
	}
	refs, found := removeRef(n.Refs, kind, target)
	if !found {
		return n, nil
	}
	n.Refs = refs
	n.Updated = e.now().UTC()
	return e.store.UpdateNote(n)
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/core/ -run 'RefEditor'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/refedit.go internal/core/refedit_test.go
git commit -m "t1: core RefEditor (task) + NoteRefEditor (note) mutations"
```

---

## Task 5: Core — `NoteRemover` (guard + `--force` + audit)

**Files:**
- Create: `internal/core/noteremove.go`
- Test: `internal/core/noteremove_test.go`

**Interfaces:**
- Consumes: `mtt.KnowledgeStore`, `mtt.AuditStore`, `mtt.AuditEntry` (`{At, Who, Why, Action, TaskID}` — reuse; `TaskID` field holds the slug string), the existing `missingAttributionFields(reqWho, reqWhy bool, by, why string) []string` and `ErrMissingAttribution` (in `internal/core`).
- Produces:
  - `type NoteRemover struct{...}`; `func NewNoteRemover(store mtt.KnowledgeStore, audit mtt.AuditStore, now func() time.Time) *NoteRemover`
  - `func (r *NoteRemover) Remove(slug mtt.NoteSlug, referents []string, force bool, by, why string) error`
    - `referents` = the incoming backlink ids the CLI computed from `Backlinks.To(RefNote, slug)` (core stays store-agnostic — it does not scan tasks).
    - `force=false`: if `len(referents)>0` → refuse `note %q is referenced by %s; use --force to delete anyway`; else `DeleteNote`.
    - `force=true`: pre-flight `missingAttributionFields(true,true,by,why)` (→ `ErrMissingAttribution`), then `audit.Append({Action:"note rm --force", TaskID: slug})` **before** `DeleteNote`.
    - A missing note (`GetNote` → `ErrNotFound`) is returned wrapped (`note %q: ErrNotFound`).

- [ ] **Step 1: Write the failing test** — `internal/core/noteremove_test.go`:

```go
package core

import (
	"errors"
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestNoteRemoverGuard(t *testing.T) {
	kb := newFakeKB()
	_, _ = kb.CreateNote(mtt.Note{Slug: "a"})
	audit := &fakeAudit{} // the package's audit fake (see remove_test.go)
	r := NewNoteRemover(kb, audit, testClock)

	// referenced + no force -> refuse, not deleted
	if err := r.Remove("a", []string{"t2"}, false, "", ""); err == nil {
		t.Fatal("referenced note must refuse without --force")
	}
	// force without who/why -> ErrMissingAttribution
	if err := r.Remove("a", []string{"t2"}, true, "", ""); !errors.Is(err, ErrMissingAttribution) {
		t.Fatalf("force needs who/why: %v", err)
	}
	// force with who/why -> audited + deleted
	if err := r.Remove("a", []string{"t2"}, true, "me", "cleanup"); err != nil {
		t.Fatal(err)
	}
	if len(audit.entries) != 1 || audit.entries[0].Action != "note rm --force" {
		t.Fatalf("audit: %+v", audit.entries)
	}
	// unreferenced -> plain delete, no force
	kb2 := newFakeKB()
	_, _ = kb2.CreateNote(mtt.Note{Slug: "b"})
	if err := NewNoteRemover(kb2, &fakeAudit{}, testClock).Remove("b", nil, false, "", ""); err != nil {
		t.Fatal(err)
	}
	// missing note -> ErrNotFound
	if err := NewNoteRemover(newFakeKB(), &fakeAudit{}, testClock).Remove("z", nil, false, "", ""); !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/core/ -run 'NoteRemoverGuard'`
Expected: FAIL — undefined symbols (and confirm the `fakeAudit`/`newFakeKB` helper names by reading `remove_test.go`/`note_test.go`; adjust if different).

- [ ] **Step 3: Implement** `internal/core/noteremove.go`:

```go
package core

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// NoteRemover deletes a note, refusing by default when other carriers reference it
// (referents supplied by the caller — core stays store-agnostic, no TaskStore). It
// mirrors Remover's dangerous-ops policy: --force forces who+why and audits before
// deleting.
type NoteRemover struct {
	store mtt.KnowledgeStore
	audit mtt.AuditStore
	now   func() time.Time
}

// NewNoteRemover wires the usecase.
func NewNoteRemover(store mtt.KnowledgeStore, audit mtt.AuditStore, now func() time.Time) *NoteRemover {
	return &NoteRemover{store: store, audit: audit, now: now}
}

// Remove deletes slug. referents are the incoming backlink ids (from Backlinks).
func (r *NoteRemover) Remove(slug mtt.NoteSlug, referents []string, force bool, by, why string) error {
	if force {
		if missing := missingAttributionFields(true, true, by, why); len(missing) > 0 {
			return fmt.Errorf("%w: %s", ErrMissingAttribution, strings.Join(missing, ", "))
		}
	}
	if _, err := r.store.GetNote(slug); err != nil {
		if errors.Is(err, mtt.ErrNotFound) {
			return fmt.Errorf("note %q: %w", slug, mtt.ErrNotFound)
		}
		return fmt.Errorf("load note %q: %w", slug, err)
	}
	if !force {
		if len(referents) > 0 {
			return fmt.Errorf("note %q is referenced by %s; use --force to delete anyway",
				slug, strings.Join(referents, ", "))
		}
		return r.store.DeleteNote(slug)
	}
	entry := mtt.AuditEntry{At: r.now().UTC().Truncate(time.Second), Who: by, Why: why, Action: "note rm --force", TaskID: mtt.TaskID(slug)}
	if err := r.audit.Append(entry); err != nil {
		return fmt.Errorf("audit append for %q: %w", slug, err)
	}
	return r.store.DeleteNote(slug)
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/core/ -run 'NoteRemoverGuard'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/noteremove.go internal/core/noteremove_test.go
git commit -m "t1: core NoteRemover — refuse-by-default guard + --force who/why/audit"
```

---

## Task 6: Core — extend task `Remover` guard with ref referents (cross-store)

**Files:**
- Modify: `internal/core/remove.go`
- Test: `internal/core/remove_test.go` (extend)

**Interfaces:**
- Consumes: `Backlinks`/`Referent` (Task 3).
- Produces (signature change):
  - `func (r *Remover) RemoveMany(ids []mtt.TaskID, force bool, by, why string, bl Backlinks) ([]RemoveResult, error)`
  - `func (r *Remover) Remove(id mtt.TaskID, force bool, by, why string, bl Backlinks) error`
  - `externalReferencingIDs(idx Index, g DepGraph, bl Backlinks, id mtt.TaskID, set map[mtt.TaskID]bool) []string` — now also appends ref-referents from `bl.To(RefTask, string(id))`, excluding task carriers in `set`; a **note** carrier is formatted `note:<slug>` (never in `set`, so it always blocks).

- [ ] **Step 1: Write the failing test** — extend `internal/core/remove_test.go`:

```go
func TestRemoverRefGuardCrossStore(t *testing.T) {
	// t5 exists; t2 (task) and note "a" both reference t5 via a ref.
	store := newMemStore(
		mtt.Task{ID: "t5"},
		mtt.Task{ID: "t2", Refs: []mtt.Ref{{Kind: mtt.RefTask, ID: "t5"}}},
	)
	notes := []mtt.Note{{Slug: "a", Refs: []mtt.Ref{{Kind: mtt.RefTask, ID: "t5"}}}}
	bl := NewBacklinks([]mtt.Task{{ID: "t5"}, {ID: "t2", Refs: []mtt.Ref{{Kind: mtt.RefTask, ID: "t5"}}}}, notes)
	r := NewRemover(store, &fakeAudit{}, testClock)

	// refuse: referenced by a task AND a note
	if err := r.Remove("t5", false, "", "", bl); err == nil {
		t.Fatal("must refuse: t5 referenced by t2 and note a")
	}
	// force deletes (leaves dangling)
	if err := r.Remove("t5", true, "me", "why", bl); err != nil {
		t.Fatalf("force: %v", err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/core/ -run 'RemoverRefGuardCrossStore'`
Expected: FAIL — `Remove`/`RemoveMany` arity mismatch (and existing callers won't compile — that is expected; fixed here + in Task 11's rm.go wiring).

- [ ] **Step 3: Implement** — in `internal/core/remove.go`:

1. Add `bl Backlinks` as the last parameter of `Remove` and `RemoveMany`; thread it into `removeOne` and `externalReferencingIDs`.
2. In `Remove`, forward: `return r.RemoveMany([]mtt.TaskID{id}, force, by, why, bl)` (then `res[0].Err`).
3. Extend `externalReferencingIDs`:

```go
func externalReferencingIDs(idx Index, g DepGraph, bl Backlinks, id mtt.TaskID, set map[mtt.TaskID]bool) []string {
	seen := map[string]bool{}
	var out []string
	addTask := func(refs []mtt.Task) {
		for _, t := range refs {
			if set[t.ID] || seen[string(t.ID)] {
				continue
			}
			seen[string(t.ID)] = true
			out = append(out, string(t.ID))
		}
	}
	addTask(idx.Children(id))
	addTask(g.Dependents(id))
	for _, ref := range bl.To(mtt.RefTask, string(id)) {
		if ref.Carrier == mtt.RefTask {
			if set[mtt.TaskID(ref.ID)] || seen[ref.ID] {
				continue
			}
			seen[ref.ID] = true
			out = append(out, ref.ID)
		} else { // note carrier — never in the deletion set; label it
			key := "note:" + ref.ID
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, key)
		}
	}
	return out
}
```

Update the two call sites in `removeOne` (pass `bl`) and the `removeOne` signature to take `bl Backlinks`.

4. **Keep the build green (critical — the signature change ripples to the CLI).** Update the two `internal/cli/rm.go` callers to pass `nil` for the new `bl` arg **in this task**: the bulk path `remover.RemoveMany(ids, force, by, why, nil)` and `runRmSingle`'s `remover.Remove(id, force, by, why, nil)`. Behavior is unchanged (`bl.To` on a nil map returns nil → no ref referents). **Task 11 replaces both `nil`s** with the real `loadBacklinks(root)` value. Without this step, `go build ./...` fails between Task 6 and Task 11.

- [ ] **Step 4: Run to verify it passes**

Run: `make check`
Expected: PASS. Existing `Remover` unit tests must also be updated to pass a `Backlinks` arg — pass `nil` where refs are irrelevant (`bl.To` on a nil map returns nil, so the structural-only tests are unaffected). The whole tree must build (the `rm.go` `nil` callers above).

- [ ] **Step 5: Commit**

```bash
git add internal/core/remove.go internal/core/remove_test.go
git commit -m "t1: extend task Remover guard with cross-store ref referents"
```

---

## Task 7: Core — creation-time refs on `Adder` + `NoteAdder`

**Files:**
- Modify: `internal/core/add.go` (`AddParams.Refs`, set in `Add`)
- Modify: `internal/core/note.go` (`NoteParams.Refs`, set in `Add`)
- Test: `internal/core/add_test.go`, `internal/core/note_test.go` (extend)

**Interfaces:**
- Produces: `AddParams.Refs []mtt.Ref` and `NoteParams.Refs []mtt.Ref`; both stored via `canonicalRefs` on create. No verification here (warn-not-block is a CLI concern).

- [ ] **Step 1: Write the failing test** — extend `internal/core/add_test.go`:

```go
func TestAdderCanonicalizesRefs(t *testing.T) {
	// build an Adder over the package fake with a minimal valid cfg (reuse the
	// existing add_test setup helper), then:
	task, err := adder.Add(AddParams{Title: "x", Refs: []mtt.Ref{
		{Kind: mtt.RefTask, ID: "t2"}, {Kind: mtt.RefNote, ID: "a"}, {Kind: mtt.RefTask, ID: "t2"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(task.Refs) != 2 { // deduped
		t.Fatalf("refs: %+v", task.Refs)
	}
	if task.Refs[0].Kind != mtt.RefNote { // sorted (note < task)
		t.Fatalf("sort: %+v", task.Refs)
	}
}
```

(Write the analogous `TestNoteAdderRefs`.)

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/core/ -run 'AdderCanonicalizesRefs|NoteAdderRefs'`
Expected: FAIL — `AddParams`/`NoteParams` have no `Refs` field.

- [ ] **Step 3: Implement**

- In `internal/core/add.go`: add `Refs []mtt.Ref` to `AddParams`; in `Add`, set the built task's `Refs` field to `canonicalRefs(p.Refs)` (only if non-empty — keep nil when empty so goldens/files are byte-unchanged: `if len(p.Refs) > 0 { task.Refs = canonicalRefs(p.Refs) }`).
- In `internal/core/note.go`: add `Refs []mtt.Ref` to `NoteParams`; in `Add`, set `Refs: canonicalRefs(p.Refs)` in the `mtt.Note{...}` literal (when empty, `canonicalRefs` returns an empty non-nil slice — guard with `if len(p.Refs) > 0` to keep nil, matching the frontmatter `omitempty` byte-identity from Task 1).

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/core/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/add.go internal/core/note.go internal/core/add_test.go internal/core/note_test.go
git commit -m "t1: creation-time refs on Adder + NoteAdder (canonicalized)"
```

---

## Task 8: CLI — `parseRefArg` + shared helpers + `mtt ref add/rm/list`

**Files:**
- Create: `internal/cli/ref.go`
- Modify: `internal/cli/root.go` (register `newRefCmd()`)
- Test: `internal/cli/ref_test.go` (unit for `parseRefArg`) + `internal/cli/testdata/scripts/ref.txt` (e2e)

**Interfaces:**
- Consumes: `core.NewRefEditor`, `core.VerifyRef`, `core.NewBacklinks`, `core.RefStatus`, `yaml.NewTaskStore`, `yaml.NewKnowledgeStore`, `taskNotFound`, `jsonFlag`, `writeJSON`, `projectRoot`, `oneID`.
- Produces:
  - `func parseRefArg(arg string) (mtt.Ref, error)` — split on first `:`; validate kind (reject `comment`, unknown); validate target per kind. Label empty (set by caller from `--label`).
  - `type refJSON struct { Kind, ID, Label, Status string }` (`label` omitempty; `status` present).
  - `func refLine(r mtt.Ref, st core.RefStatus) string` — `note:auth-design  [ok]  (label)`.
  - `func backlinkLine(ref core.Referent, targetKind mtt.RefKind) string`.
  - Helpers `taskExistsFn(tasks []mtt.Task) func(mtt.TaskID) bool`, `noteExistsFn(notes []mtt.Note) func(mtt.NoteSlug) bool`.

- [ ] **Step 1: Write the failing unit test** — `internal/cli/ref_test.go`:

```go
package cli

import (
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestParseRefArg(t *testing.T) {
	ok := []struct {
		in   string
		kind mtt.RefKind
		id   string
	}{
		{"task:t2", mtt.RefTask, "t2"},
		{"note:auth-design", mtt.RefNote, "auth-design"},
		{"url:https://a/b:c?x=1", mtt.RefURL, "https://a/b:c?x=1"}, // split on FIRST colon
	}
	for _, c := range ok {
		got, err := parseRefArg(c.in)
		if err != nil || got.Kind != c.kind || got.ID != c.id {
			t.Fatalf("%q: got %+v err=%v", c.in, got, err)
		}
	}
	bad := []string{"task", "comment:t2#1", "bogus:x", "url:example.com", "task:", "note:Bad_Slug"}
	for _, in := range bad {
		if _, err := parseRefArg(in); err == nil {
			t.Fatalf("%q must error", in)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/cli/ -run 'ParseRefArg'`
Expected: FAIL — undefined `parseRefArg`.

- [ ] **Step 3: Implement** `internal/cli/ref.go` (parser + helpers + command group):

```go
package cli

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// parseRefArg parses "<kind>:<target>" (split on the FIRST colon — a url has more).
// kind must be note/task/url; comment is rejected (t2); each target is validated by
// its own identity rule. Label is empty (the caller sets it from --label).
func parseRefArg(arg string) (mtt.Ref, error) {
	k, target, found := strings.Cut(arg, ":")
	if !found {
		return mtt.Ref{}, fmt.Errorf("expected <kind>:<target> (example: task:t2), got %q", arg)
	}
	kind := mtt.RefKind(k)
	if kind == mtt.RefComment {
		return mtt.Ref{}, fmt.Errorf("comments arrive in t2; the 'comment' ref kind is not yet supported")
	}
	switch kind {
	case mtt.RefTask:
		if _, err := mtt.NewTaskID(target); err != nil {
			return mtt.Ref{}, fmt.Errorf("invalid task target %q: %w", target, err)
		}
	case mtt.RefNote:
		if _, err := mtt.NewNoteSlug(target); err != nil {
			return mtt.Ref{}, fmt.Errorf("invalid note target %q: %w", target, err)
		}
	case mtt.RefURL:
		u, err := url.Parse(target)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return mtt.Ref{}, fmt.Errorf("invalid url target %q (need scheme and host, e.g. https://example.com/x)", target)
		}
	default:
		return mtt.Ref{}, fmt.Errorf("unknown ref kind %q (want note|task|url)", k)
	}
	return mtt.Ref{Kind: kind, ID: target}, nil
}

type refJSON struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Label  string `json:"label,omitempty"`
	Status string `json:"status"`
}

func toRefJSON(r mtt.Ref, st core.RefStatus) refJSON {
	return refJSON{Kind: string(r.Kind), ID: r.ID, Label: r.Label, Status: string(st)}
}

func refLine(r mtt.Ref, st core.RefStatus) string {
	s := fmt.Sprintf("%s:%s  [%s]", r.Kind, r.ID, st)
	if r.Label != "" {
		s += "  (" + r.Label + ")"
	}
	return s
}

func taskExistsFn(tasks []mtt.Task) func(mtt.TaskID) bool {
	set := make(map[mtt.TaskID]bool, len(tasks))
	for _, t := range tasks {
		set[t.ID] = true
	}
	return func(id mtt.TaskID) bool { return set[id] }
}

func noteExistsFn(notes []mtt.Note) func(mtt.NoteSlug) bool {
	set := make(map[mtt.NoteSlug]bool, len(notes))
	for _, n := range notes {
		set[n.Slug] = true
	}
	return func(s mtt.NoteSlug) bool { return set[s] }
}

// verifyOne resolves one ref against the two stores (KB always wired in YAML) — for
// the single-op warn on write.
func verifyOne(root string, r mtt.Ref) core.RefStatus {
	tasks, _ := yaml.NewTaskStore(root).List()
	notes, _ := yaml.NewKnowledgeStore(root).ListNotes()
	return core.VerifyRef(r, taskExistsFn(tasks), noteExistsFn(notes))
}

// warnIfNotOK warns (stderr, warn-not-block) about a DANGLING ref, or a note ref
// that could not be verified (no KB wired). A well-formed url is expected to be
// unverified and does NOT warn (DESIGN: "warn about a *dangling* reference").
func warnIfNotOK(cmd *cobra.Command, r mtt.Ref, st core.RefStatus) {
	if st == core.RefDangling || (st == core.RefUnverified && r.Kind == mtt.RefNote) {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s:%s is %s\n", r.Kind, r.ID, st)
	}
}

// newRefCmd builds `mtt ref` with add/rm/list (the dep pattern) for TASK carriers.
func newRefCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "ref", Short: "Manage references on tasks (note/task/url)"}
	cmd.AddCommand(newRefAddCmd(), newRefRmCmd(), newRefListCmd())
	return cmd
}

func newRefAddCmd() *cobra.Command {
	var label string
	cmd := &cobra.Command{
		Use:   "add <id> <kind>:<target>",
		Short: "Add a reference to a task",
		Args:  twoIDs("provide a task id and <kind>:<target> (example: mtt ref add t2 task:t1)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			id, err := mtt.NewTaskID(args[0])
			if err != nil {
				return err
			}
			ref, err := parseRefArg(args[1])
			if err != nil {
				return err
			}
			ref.Label = label
			task, err := core.NewRefEditor(yaml.NewTaskStore(root), time.Now).AddRef(id, ref, cmd.Flags().Changed("label"))
			if err != nil {
				if isNotFound(err) {
					return taskNotFound(id)
				}
				return err
			}
			st := verifyOne(root, ref)
			warnIfNotOK(cmd, ref, st)
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), toRefJSON(ref, st))
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "added %s:%s to %s\n", ref.Kind, ref.ID, task.ID)
			return err
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "annotate the reference")
	return cmd
}

func newRefRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id> <kind>:<target>",
		Short: "Remove a reference from a task (idempotent)",
		Args:  twoIDs("provide a task id and <kind>:<target> (example: mtt ref rm t2 task:t1)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			id, err := mtt.NewTaskID(args[0])
			if err != nil {
				return err
			}
			ref, err := parseRefArg(args[1])
			if err != nil {
				return err
			}
			if _, err := core.NewRefEditor(yaml.NewTaskStore(root), time.Now).RemoveRef(id, ref.Kind, ref.ID); err != nil {
				if isNotFound(err) {
					return taskNotFound(id)
				}
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "removed %s:%s from %s\n", ref.Kind, ref.ID, id)
			return err
		},
	}
}

func newRefListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <id>",
		Short: "List a task's references and backlinks",
		Args:  oneID("provide a task id (example: mtt ref list t2)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			id, err := mtt.NewTaskID(args[0])
			if err != nil {
				return err
			}
			store := yaml.NewTaskStore(root)
			task, err := store.Get(id)
			if err != nil {
				if isNotFound(err) {
					return taskNotFound(id)
				}
				return err
			}
			tasks, err := store.List()
			if err != nil {
				return err
			}
			notes, err := yaml.NewKnowledgeStore(root).ListNotes()
			if err != nil {
				return err
			}
			return writeRefsAndBacklinks(cmd, mtt.RefTask, string(id), task.Refs, tasks, notes)
		},
	}
}
```

Add the shared renderer at the bottom of `ref.go` (reused by `note ref list` and `show`):

```go
// backlinkJSON is one incoming backlink (carrier kind + id + the forward ref's
// label). Reused by ref list / note ref list / show / note show.
type backlinkJSON struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
}

func verifiedRefsJSON(refs []mtt.Ref, te func(mtt.TaskID) bool, ne func(mtt.NoteSlug) bool) []refJSON {
	out := make([]refJSON, 0, len(refs))
	for _, r := range refs {
		out = append(out, toRefJSON(r, core.VerifyRef(r, te, ne)))
	}
	return out
}

func toBacklinkJSON(rs []core.Referent) []backlinkJSON {
	out := make([]backlinkJSON, 0, len(rs))
	for _, r := range rs {
		out = append(out, backlinkJSON{Kind: string(r.Carrier), ID: r.ID, Label: r.Label})
	}
	return out
}

// formatRefsBacklinks renders the human refs:/backlinks: block for show / note show,
// or "" when the carrier has neither (so a ref-less show is byte-unchanged). refs
// are indented under a 2-space header to match formatTask's field style.
func formatRefsBacklinks(refs []mtt.Ref, back []core.Referent, te func(mtt.TaskID) bool, ne func(mtt.NoteSlug) bool) string {
	if len(refs) == 0 && len(back) == 0 {
		return ""
	}
	var b strings.Builder
	if len(refs) > 0 {
		b.WriteString("  refs:\n")
		for _, r := range refs {
			fmt.Fprintf(&b, "    %s\n", refLine(r, core.VerifyRef(r, te, ne)))
		}
	}
	if len(back) > 0 {
		b.WriteString("  backlinks:\n")
		for _, r := range back {
			if r.Label != "" {
				fmt.Fprintf(&b, "    %s:%s  (%s)\n", r.Carrier, r.ID, r.Label)
			} else {
				fmt.Fprintf(&b, "    %s:%s\n", r.Carrier, r.ID)
			}
		}
	}
	return b.String()
}

// writeRefsAndBacklinks renders a carrier's outgoing refs (verified) + incoming
// backlinks for `ref list` / `note ref list`, in text (always both headers) or
// (--json) {refs:[...], backlinks:[...]} (both non-null).
func writeRefsAndBacklinks(cmd *cobra.Command, carrierKind mtt.RefKind, carrierID string, refs []mtt.Ref, tasks []mtt.Task, notes []mtt.Note) error {
	te, ne := taskExistsFn(tasks), noteExistsFn(notes)
	back := core.NewBacklinks(tasks, notes).To(carrierKind, carrierID)
	if jsonFlag(cmd) {
		return writeJSON(cmd.OutOrStdout(), struct {
			Refs      []refJSON      `json:"refs"`
			Backlinks []backlinkJSON `json:"backlinks"`
		}{verifiedRefsJSON(refs, te, ne), toBacklinkJSON(back)})
	}
	var b strings.Builder
	b.WriteString("refs:\n")
	for _, r := range refs {
		fmt.Fprintf(&b, "  %s\n", refLine(r, core.VerifyRef(r, te, ne)))
	}
	b.WriteString("backlinks:\n")
	for _, r := range back {
		if r.Label != "" {
			fmt.Fprintf(&b, "  %s:%s  (%s)\n", r.Carrier, r.ID, r.Label)
		} else {
			fmt.Fprintf(&b, "  %s:%s\n", r.Carrier, r.ID)
		}
	}
	_, err := fmt.Fprint(cmd.OutOrStdout(), b.String())
	return err
}
```

Add `isNotFound` to `internal/cli/errors.go` if not present:

```go
func isNotFound(err error) bool { return errors.Is(err, mtt.ErrNotFound) }
```

Register in `internal/cli/root.go`: add `newRefCmd()` to the `AddCommand(...)` list (find the existing list that includes `newDepCmd()`/`newNoteCmd()`).

- [ ] **Step 4: Write the e2e** `internal/cli/testdata/scripts/ref.txt` (txtar). Cover: add task ref (ok), backlink appears on the target, add a dangling task ref (`ref add … task:t999` → warns, exit 0, status `dangling`), `ref rm` twice (both exit 0), `comment:t2#1` rejected (exit 1). Mirror an existing script (e.g. `dep.txt`/`note.txt`) for the `exec mtt …`/`stdout`/`stderr`/`! exec` idioms and the `env MTT_DIR`/`mtt init` preamble. **Note (D5):** a missing **note** target with the KB wired (always, in YAML) resolves to `dangling`, NOT `unverified` — do not assert `unverified` for a missing note (spec AC-4's loose wording is superseded by D5). `unverified` is only `url` (and note-with-no-KB, unreachable via the YAML CLI).

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run 'ParseRefArg|Script/ref'`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/ref.go internal/cli/ref_test.go internal/cli/errors.go internal/cli/root.go internal/cli/testdata/scripts/ref.txt
git commit -m "t1: mtt ref add/rm/list + parseRefArg + shared ref/backlink renderer"
```

---

## Task 9: CLI — `mtt note ref` group + `--ref` on `add`/`note add`

**Files:**
- Create: `internal/cli/noteref.go`
- Modify: `internal/cli/note.go` (wire `newNoteRefCmd()` into `newNoteCmd`; add `--ref` to `note add`)
- Modify: `internal/cli/add.go` (add `--ref`)
- Test: `internal/cli/testdata/scripts/noteref.txt`, extend `ref.txt` for creation-time `--ref`

**Interfaces:**
- Consumes: `core.NewNoteRefEditor`, `parseRefArg`, `writeRefsAndBacklinks`, `verifyOne`, `warnIfNotOK`, `noteNotFound`, `core.AddParams.Refs`/`core.NoteParams.Refs` (Task 7).
- Produces: `func newNoteRefCmd() *cobra.Command`; a shared `parseRefFlags(vals []string) ([]mtt.Ref, error)` for `--ref`.

- [ ] **Step 1: Write the e2e first** `internal/cli/testdata/scripts/noteref.txt`: `note add auth-design`; `note ref add auth-design task:t2` (t2 exists → ok); `note ref list auth-design` shows the ref; `show t2` shows a backlink from the note; `note ref rm auth-design task:t2` (idempotent second call exits 0). Also add to `ref.txt`: `add "x" --ref task:t1` then `ref list <newid>` shows it. Mirror `note.txt` idioms.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/cli/ -run 'Script/noteref'`
Expected: FAIL — `mtt note ref` is an unknown command.

- [ ] **Step 3: Implement**

`internal/cli/noteref.go`:

```go
package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// parseRefFlags parses repeated --ref <kind>:<target> values (creation-time).
func parseRefFlags(vals []string) ([]mtt.Ref, error) {
	out := make([]mtt.Ref, 0, len(vals))
	for _, v := range vals {
		r, err := parseRefArg(v)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func newNoteRefCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "ref", Short: "Manage references on a note"}
	cmd.AddCommand(newNoteRefAddCmd(), newNoteRefRmCmd(), newNoteRefListCmd())
	return cmd
}

func newNoteRefAddCmd() *cobra.Command {
	var label string
	cmd := &cobra.Command{
		Use:   "add <slug> <kind>:<target>",
		Short: "Add a reference to a note",
		Args:  twoIDs("provide a note slug and <kind>:<target> (example: mtt note ref add a task:t1)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			slug, err := mtt.NewNoteSlug(args[0])
			if err != nil {
				return err
			}
			ref, err := parseRefArg(args[1])
			if err != nil {
				return err
			}
			ref.Label = label
			note, err := core.NewNoteRefEditor(yaml.NewKnowledgeStore(root), time.Now).AddRef(slug, ref, cmd.Flags().Changed("label"))
			if err != nil {
				if isNotFound(err) {
					return noteNotFound(slug)
				}
				return err
			}
			st := verifyOne(root, ref)
			warnIfNotOK(cmd, ref, st)
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), toRefJSON(ref, st))
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "added %s:%s to %s\n", ref.Kind, ref.ID, note.Slug)
			return err
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "annotate the reference")
	return cmd
}
// newNoteRefRmCmd + newNoteRefListCmd mirror newRefRmCmd/newRefListCmd but over the
// KnowledgeStore (slug via NewNoteSlug, noteNotFound on miss, NoteRefEditor.RemoveRef,
// and writeRefsAndBacklinks(cmd, mtt.RefNote, string(slug), note.Refs, tasks, notes)).
```

Write `newNoteRefRmCmd` and `newNoteRefListCmd` in full (repeat the structure — the implementer may read tasks out of order; use `noteNotFound(slug)`, `core.NewNoteRefEditor(...).RemoveRef`, and for list: `yaml.NewKnowledgeStore(root).GetNote(slug)` → `store.ListNotes()` + `yaml.NewTaskStore(root).List()` → `writeRefsAndBacklinks(cmd, mtt.RefNote, string(slug), note.Refs, tasks, notes)`).

Wire into `newNoteCmd` in `note.go`: add `newNoteRefCmd()` to its `AddCommand(...)`.

`--ref` flag on `note add` (in `newNoteAddCmd`, `note.go`) and `add` (in `newAddCmd`, `add.go`):

```go
// add.go, in newAddCmd var block: refVals []string
cmd.Flags().StringArrayVar(&refVals, "ref", nil, "add a reference <kind>:<target> (repeatable)")
// in RunE, before adder.Add:
refs, err := parseRefFlags(refVals)
if err != nil {
	return err
}
// ...pass Refs: refs into core.AddParams{...}
// after a successful Add, warn on any non-ok ref:
for _, r := range refs {
	warnIfNotOK(cmd, r, verifyOne(root, r))
}
```

Apply the same three edits (var, flag, parse+pass `Refs`+warn) to `newNoteAddCmd` with `core.NoteParams{... Refs: refs}`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run 'Script/(noteref|ref)'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/noteref.go internal/cli/note.go internal/cli/add.go internal/cli/testdata/scripts/noteref.txt internal/cli/testdata/scripts/ref.txt
git commit -m "t1: mtt note ref group + creation-time --ref on add/note add"
```

---

## Task 10: CLI — `mtt check`

**Files:**
- Create: `internal/cli/check.go`
- Modify: `internal/cli/root.go` (register `newCheckCmd()`; map `core.ErrDanglingRefs`→7 in `exitCode`)
- Test: `internal/cli/testdata/scripts/check.txt`

**Interfaces:**
- Consumes: `core.CheckRefs`, `core.CheckFinding`, `core.RefDangling`, `core.ErrDanglingRefs`.
- Produces: `func newCheckCmd() *cobra.Command`; exit 7 when any dangling.

- [ ] **Step 1: Write the failing tests** — TWO tests, because `testscript`'s `! exec` asserts only **non-zero**, not a specific code (the repo says so itself: `testdata/scripts/tags.txt` — "the exact exit-4 mapping is a unit test"). Exit-code mappings are unit-tested in `TestExitCode` (`internal/cli/status_test.go`).

  (a) **Unit — the exit-7 mapping.** Add a case to the existing `TestExitCode` table (`internal/cli/status_test.go`): `{err: core.ErrDanglingRefs, want: 7}` (match the table's actual field names — read the test first). This is what actually guards exit 7 against a regression to 1.

  (b) **e2e** `internal/cli/testdata/scripts/check.txt`: clean repo → `exec mtt check` (exit 0); add a dangling ref (`ref add t1 task:t999`) → `! exec mtt check` (**non-zero** — do not assert the number here) and assert the printed dangling line via `stdout 't1.*t999.*dangling'`; a url-only ref present → `exec mtt check` still **exit 0** (unverified is not a failure).

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/cli/ -run 'TestExitCode|Script/check'`
Expected: FAIL — `core.ErrDanglingRefs` undefined / mapping absent / unknown command `check`.

- [ ] **Step 3: Implement** `internal/cli/check.go`:

```go
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newCheckCmd builds `mtt check`: a read-only repo-wide reference integrity sweep.
// Exit 7 when any dangling reference is found; unverified (url / no-KB) is not a
// failure.
func newCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Verify references across the repository (exit 7 on dangling)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			tasks, err := yaml.NewTaskStore(root).List()
			if err != nil {
				return err
			}
			notes, err := yaml.NewKnowledgeStore(root).ListNotes()
			if err != nil {
				return err
			}
			findings := core.CheckRefs(tasks, notes, true) // YAML KB always wired
			dangling := 0
			for _, f := range findings {
				if f.Status == core.RefDangling {
					dangling++
				}
			}
			if jsonFlag(cmd) {
				out := make([]refCheckJSON, 0, len(findings))
				for _, f := range findings {
					out = append(out, toRefCheckJSON(f))
				}
				if err := writeJSON(cmd.OutOrStdout(), out); err != nil {
					return err
				}
			} else {
				var b strings.Builder
				for _, f := range findings {
					fmt.Fprintf(&b, "%s:%s → %s:%s   [%s]\n", f.CarrierKind, f.CarrierID, f.Ref.Kind, f.Ref.ID, f.Status)
				}
				fmt.Fprintf(&b, "%d dangling, %d unverified across %d entities\n", dangling, len(findings)-dangling, countCarriers(findings))
				if _, err := fmt.Fprint(cmd.OutOrStdout(), b.String()); err != nil {
					return err
				}
			}
			if dangling > 0 {
				return core.ErrDanglingRefs
			}
			return nil
		},
	}
}

// refCheckJSON is the `mtt check --json` shape: carrier + ref + status. NOTE the
// name is refCheckJSON, NOT checkJSON — json.go already has a checkJSON (the gate
// command-result view); reusing it would collide. (Confirmed: grep -n "checkJSON"
// internal/cli/*.go shows json.go:75.)
type refCheckJSON struct {
	Carrier struct {
		Kind string `json:"kind"`
		ID   string `json:"id"`
	} `json:"carrier"`
	Ref    refJSON `json:"ref"`
	Status string  `json:"status"`
}

func toRefCheckJSON(f core.CheckFinding) refCheckJSON {
	var j refCheckJSON
	j.Carrier.Kind, j.Carrier.ID = string(f.CarrierKind), f.CarrierID
	j.Ref = refJSON{Kind: string(f.Ref.Kind), ID: f.Ref.ID, Label: f.Ref.Label, Status: string(f.Status)}
	j.Status = string(f.Status)
	return j
}

func countCarriers(fs []core.CheckFinding) int {
	seen := map[string]bool{}
	for _, f := range fs {
		seen[string(f.CarrierKind)+":"+f.CarrierID] = true
	}
	return len(seen)
}
```

In `internal/cli/root.go`: register `newCheckCmd()` in the command list, and add to `exitCode`:

```go
case errors.Is(err, core.ErrDanglingRefs):
	return 7
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run 'TestExitCode|Script/check'`
Expected: PASS — the exit-7 mapping is proven by `TestExitCode`; the e2e proves the sweep prints the dangling line and exits non-zero.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/check.go internal/cli/root.go internal/cli/status_test.go internal/cli/testdata/scripts/check.txt
git commit -m "t1: mtt check — read-only ref integrity sweep, exit 7 on dangling"
```

---

## Task 11: CLI — Refs/Backlinks in `show`/`note show`; task `rm` + `note rm` guards

**Files:**
- Modify: `internal/cli/show.go` (`formatTask` refs/backlinks; wire snapshot), `internal/cli/json.go` (`showJSON` refs/backlinks)
- Modify: `internal/cli/note.go` (`writeNote` + `noteJSON` refs/backlinks; `note rm` guard + `--force`)
- Modify: `internal/cli/rm.go` (build + pass `Backlinks` into `Remover`)
- Test: extend `internal/cli/testdata/scripts/{show,ref,noteref,rm,note}.txt`

**Interfaces:**
- Consumes: `core.NewBacklinks`, `core.NewNoteRemover`, `core.Remover` (new `bl` param, Task 6), `resolveAttribution`, `yaml.NewAuditStore`.

- [ ] **Step 1: Write the failing e2e** — (`! exec` asserts **non-zero**; the specific 2/4 codes are already covered by `TestExitCode`/`TestExitCodeNotFound`, so assert non-zero + the message here, not the number). Extend `rm.txt`: `ref add t2 task:t1`; `! exec mtt rm t1` (refused — assert `stderr 'referenced by t2'`, t1 still present); `! exec mtt rm t1 --force` (non-zero — missing who/why pre-flight); `exec mtt rm t1 --force --who me --why x` (deletes). Extend `note.txt`: `ref add t2 note:a`; `! exec mtt note rm a` (refused — assert `stderr 'referenced by'`); `! exec mtt note rm a --force` (non-zero — missing who/why); `exec mtt note rm a --force --who me --why x` (deletes); `! exec mtt note rm missing` (non-zero — not found). Extend `show.txt`: after `ref add`, `exec mtt show t2` shows a `refs:` section and `show t1` a `backlinks:` section.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/cli/ -run 'Script/(rm|note|show)'`
Expected: FAIL — no guard on rm/note rm; no refs/backlinks in show. (Also the `rm.go`/`runRmSingle` calls won't compile until the `Backlinks` arg is threaded — Task 6 changed the signature.)

- [ ] **Step 3: Implement**

**rm.go** — build `Backlinks` from tasks+notes and pass it to `Remove`/`RemoveMany`:

```go
// helper (add to rm.go): snapshot both stores → Backlinks
func loadBacklinks(root string) (core.Backlinks, error) {
	tasks, err := yaml.NewTaskStore(root).List()
	if err != nil {
		return nil, err
	}
	notes, err := yaml.NewKnowledgeStore(root).ListNotes()
	if err != nil {
		return nil, err
	}
	return core.NewBacklinks(tasks, notes), nil
}
```

In the bulk path: `bl, err := loadBacklinks(root)` (before `RemoveMany`), then `remover.RemoveMany(ids, force, by, why, bl)`. In `runRmSingle`: `bl, err := loadBacklinks(root)` then `remover.Remove(id, force, by, why, bl)`.

**note.go `newNoteRmCmd`** — replace the unconditional delete with the guard + `--force`:

```go
func newNoteRmCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "rm <slug>",
		Short: "Delete a knowledge note (refuses if referenced; --force overrides)",
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
			kb := yaml.NewKnowledgeStore(root)
			var note mtt.Note
			if jsonFlag(cmd) {
				if note, err = kb.GetNote(slug); err != nil {
					if isNotFound(err) {
						return noteNotFound(slug)
					}
					return err
				}
			}
			tasks, err := yaml.NewTaskStore(root).List()
			if err != nil {
				return err
			}
			notes, err := kb.ListNotes()
			if err != nil {
				return err
			}
			referents := referentIDs(core.NewBacklinks(tasks, notes).To(mtt.RefNote, string(slug)), slug)
			_, settings, err := yaml.Load(root)
			if err != nil {
				return err
			}
			_, by, why, err := resolveAttribution(cmd, settings.Author)
			if err != nil {
				return err
			}
			if err := core.NewNoteRemover(kb, yaml.NewAuditStore(root), time.Now).Remove(slug, referents, force, by, why); err != nil {
				if isNotFound(err) {
					return noteNotFound(slug)
				}
				return err // ErrMissingAttribution → exit 2; referenced-refusal → exit 1
			}
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), toNoteJSON(note))
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", slug)
			return err
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "delete even if referenced (leaves dangling refs)")
	return cmd
}

// referentIDs formats backlink referents as strings (note carriers labelled),
// EXCLUDING the note's own self-reference (a note referencing itself must not block
// its own delete — symmetric with the task guard's subgraph-ignore of the deletion set).
func referentIDs(refs []core.Referent, self mtt.NoteSlug) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		if r.Carrier == mtt.RefNote {
			if r.ID == string(self) {
				continue // self-ref never blocks
			}
			out = append(out, "note:"+r.ID)
		} else {
			out = append(out, r.ID)
		}
	}
	return out
}
```

**json.go** — add two fields to `showJSON` (do **NOT** change `toShowJSON`'s signature — its 5 callers in `show_json_test.go` must keep compiling; set the fields at the call site instead):

```go
type showJSON struct {
	taskJSON
	StatusDescription string         `json:"status_description,omitempty"`
	Next              []nextMoveJSON `json:"next,omitempty"`
	History           []historyJSON  `json:"history,omitempty"`
	Refs              []refJSON      `json:"refs,omitempty"`
	Backlinks         []backlinkJSON `json:"backlinks,omitempty"`
}
```

**show.go** — restructure `newShowCmd`'s RunE so `tasks`+`notes` load **before** the `jsonFlag` branch (both paths need them), then populate refs/backlinks. `taskJSON` (used by `list`/`edit`/`status --json`) stays untouched:

```go
tasks, err := store.List()
if err != nil {
	return err
}
notes, err := yaml.NewKnowledgeStore(root).ListNotes()
if err != nil {
	return err
}
te, ne := taskExistsFn(tasks), noteExistsFn(notes)
back := core.NewBacklinks(tasks, notes).To(mtt.RefTask, string(task.ID))
if jsonFlag(cmd) {
	sj := toShowJSON(task, statusDesc, onward) // UNCHANGED 3-arg signature
	sj.Refs = verifiedRefsJSON(task.Refs, te, ne)
	sj.Backlinks = toBacklinkJSON(back)
	return writeJSON(cmd.OutOrStdout(), sj)
}
idx := core.NewIndex(tasks)
out := formatTask(task, idx.Ancestors(task.ID), idx.Children(task.ID), statusDesc, onward)
out += formatRefsBacklinks(task.Refs, back, te, ne) // "" when both empty → byte-unchanged for a ref-less task
_, err = fmt.Fprint(cmd.OutOrStdout(), out)
return err
```

Note: `sj.Refs`/`sj.Backlinks` use `omitempty` on **nil** slices, but `verifiedRefsJSON`/`toBacklinkJSON` return **non-nil empty** slices (`make(..., 0, 0)`). To keep a ref-less task's `show --json` byte-identical, guard: assign only when non-empty (`if len(task.Refs) > 0 { sj.Refs = ... }`, same for `back`).

**note.go `writeNote` / `newNoteShowCmd`** — the note analogue: define `type noteShowJSON struct { noteJSON; Refs []refJSON `json:"refs,omitempty"`; Backlinks []backlinkJSON `json:"backlinks,omitempty"` }` (keep the lean `noteJSON` for `note list`/`add`/`edit`). In `newNoteShowCmd`, load `tasks`+`notes`, build `back := core.NewBacklinks(tasks, notes).To(mtt.RefNote, string(slug))`; JSON path emits `noteShowJSON` with the same non-empty guards; human path appends `formatRefsBacklinks(note.Refs, back, te, ne)` after `writeNote`'s body output.

- [ ] **Step 4: Run the full suite**

Run: `make check`
Expected: PASS (all packages, `-race`). Fix any golden/order drift.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/rm.go internal/cli/note.go internal/cli/show.go internal/cli/json.go internal/cli/testdata/scripts/
git commit -m "t1: rm/note-rm ref guards + refs/backlinks in show & note show"
```

---

## Task 12: Docs sync

**Files (all Modify):**
- `CLI_REFERENCE.md` ↔ `CLI_REFERENCE.ru.md`
- `DESIGN.md` ↔ `DESIGN.ru.md`
- `docs/architecture/model.go`
- `internal/core/CLAUDE.md`, `internal/cli/CLAUDE.md`, `internal/adapter/yaml/CLAUDE.md`, `pkg/mtt/CLAUDE.md`
- `AGENTS.md` (only if a convention changed — it should not)

**No test cycle** (docs). Follow the spec's "Docs to sync" section verbatim. Grep every parallel occurrence (EN + RU) before editing.

- [ ] **Step 1:** `CLI_REFERENCE.md` References section — drop the phase markers; document `mtt ref add/rm/list`, `mtt note ref`, creation-time `--ref`, the `(kind,target)` key, statuses, exit codes; **correct** the sketch line "`Exits 4 if no such reference exists`" → idempotent no-op (exit 0); update `mtt check` (drop "phase 5", pin exit 7, `--fix` deferred); update `mtt note rm` (refuse+`--force`+who/why/audit); note `comment` refs are t2. Mirror into `CLI_REFERENCE.ru.md`.

- [ ] **Step 2:** `DESIGN.md` §"Knowledge base and references" — Phases bullet + "KB & refs" decision row (refs wired in t1 for note/task/url; comment→t2); add a "**Shipped (t1)**" block (kinds, carriers, computed cross-store backlinks, `check` exit 7, refuse-guard); reaffirm "back-refs computed, never stored"; **reconcile** the stale s008.5 `mtt rm` sentence ("`--who`/`--why` are not mandated…" — superseded by t5, now extended to `note rm`). Mirror into `DESIGN.ru.md`.

- [ ] **Step 3:** `docs/architecture/model.go` — add `Refs []Ref` to the `Note` block; note `RefStatus`/`Backlinks` are core-derived (not contract).

- [ ] **Step 4:** CLAUDE.md files — `pkg/mtt` (`Note` no longer refs-free); `internal/core` (ref-set algebra, `VerifyRef`/`RefStatus`, `Backlinks`, `CheckRefs`, `RefEditor`/`NoteRefEditor`, `NoteRemover`, extended `Remover` signature); `internal/cli` (`ref`/`note ref`/`check`, `--ref`, refs/backlinks in show, note-rm guard); `internal/adapter/yaml` (note frontmatter `refs`). Keep each thin.

- [ ] **Step 5: Verify + commit**

```bash
make check
git add -A
git commit -m "t1: docs sync — CLI_REFERENCE/DESIGN (EN+RU), model.go, CLAUDE.md files"
```

---

## Self-review (completed before submit)

- **Spec coverage:** D1 kinds/carriers → T1,T7,T8; D2 Note.Refs → T1; D3 identity/sort/idempotent-rm → T2,T4,T8; D4 CLI groups + `--ref` + parse → T8,T9; D5 verify/RefStatus → T2,T3; D6 warn-not-block → T8,T9; D7 Backlinks → T3,T11; D8 check/exit-7 → T10; D9 deletion guard (task+note, cross-store) → T5,T6,T11; D10 exit codes → T8,T10,T11; D11 JSON shapes → T8,T10,T11. All covered.
- **Type consistency:** `RefStatus`/`RefOK`/`RefDangling`/`RefUnverified`, `Backlinks`/`Referent`/`RefKey`, `CheckFinding`, `RefEditor`/`NoteRefEditor`/`NoteRemover`, and the `Remover.Remove/RemoveMany` new `bl Backlinks` param are used consistently across T2–T11. The `mtt check` JSON type is `refCheckJSON` (NOT `checkJSON` — avoids the `json.go:75` clash). `toShowJSON` keeps its 3-arg signature (its `show_json_test.go` callers unaffected); `show.go` sets `sj.Refs`/`sj.Backlinks` at the call site. `backlinkJSON`/`verifiedRefsJSON`/`toBacklinkJSON`/`formatRefsBacklinks` (T8) are shared by `ref list`/`note ref list`/`show`/`note show`.
- **Build-green per task:** T6 changes `Remover.Remove/RemoveMany` and immediately updates the `rm.go` callers to pass `nil` (real `Backlinks` wired in T11) + the `remove_test.go` call sites, so every commit builds. No other cross-task signature ripple.
- **Exit-code coverage:** exit 7 is proven by a `TestExitCode` unit case (T10) — `! exec` only asserts non-zero, so the number is never left to an e2e.
- **Placeholder scan:** none — every code step carries real code; the two "mirror the sibling" spots (T9 note-ref rm/list, T11 note-show JSON) name the exact functions and signatures to repeat.
- **Confirmed against code (names pinned, not guessed):** `internal/core` test fakes are `newMemStore(...tasks)` (TaskStore), `newFakeKB()` seeded via `CreateNote` (KnowledgeStore), `fakeAudit{failOnID}` (AuditStore), clock `testClock`; e2e scripts live in `internal/cli/testdata/scripts/`; `missingAttributionFields(reqWho, reqWhy bool, by, why string) []string` (`internal/core/transition.go:146`).
