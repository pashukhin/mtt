# KB prime (t51) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A curated, opt-in, pointer-only KB digest for session-start injection — `Note.Priority` (reusing the task Priority VO) + a backlink-count tiebreak, and a pure-read `mtt prime` command.

**Architecture:** Hexagonal, unchanged. Reuse the task `Priority` VO (`pkg/mtt`) on `Note`, the s008.6 filter/sort machinery (`core.ListFilter.Sort`/`lessByPriority`), and t1's computed `core.Backlinks`. Add a pure `core.Prime` derived read (like `Roadmap`, not in the contract) and a thin `mtt prime` CLI. No new ports. The `sessionStart` hook is config/docs, not code.

**Tech Stack:** Go 1.23+, cobra CLI, `gopkg.in/yaml.v3`, `testscript` e2e, table-driven unit tests, golden files.

## Global Constraints

- **Spec of record:** `docs/superpowers/specs/t51-kb-prime.md` — decisions D1–D9 are binding.
- **TDD:** red → green → refactor; failing test first. `make check` (gofmt + vet + golangci-lint v2 + `go test -race -cover` + build) green before every commit.
- **Layering:** `core` imports no `adapter/*`; `pkg/mtt` domain carries no yaml/json tags; `Prime`/`PrimeEntry` are `core`-derived, **not** in the `pkg/mtt` contract (like `Roadmap`/`Backlinks`).
- **Reuse, don't reinvent:** the `Priority` VO (`pkg/mtt/priority.go` — `PriorityHigh/Medium/Low`, `Rank()` high=0/medium=1/low=2, `Valid()`), `parsePriority`/`toPriorities` (`internal/cli/priority.go`), the `ListFilter.Sort`+`lessByPriority` pattern (`internal/core/list.go`), the task `edit --priority` precedent (`*mtt.Priority` + `Changed("priority")` + the nothing-to-edit guard), and `Backlinks.To(RefNote, slug)` (t1).
- **Opt-in safety (D7):** a note is `prime`-eligible **iff** it has an **explicit** priority (`Priority != ""`) whose `Rank() <= MinPriority.Rank()`. **Unset notes are never primed.** Bodies are never emitted (pointer-only). The digest is capped by `--limit`.
- **Byte-identity:** a note without a priority round-trips + `show`s byte-identically to pre-t51 (`omitempty`, nil/empty guards).
- **Footer invariant (D4):** `(<N> of <M> important notes shown — …)` where **M** = eligible-at-threshold before `--limit`, **N** = shown after. Uncapped default → `N of N`.
- **Docs bilingual** (EN+RU) where applicable: DESIGN, CLI_REFERENCE. Grep all parallel occurrences before editing.

---

## File structure

**Create:**
- `internal/core/prime.go` — `PrimeOptions`, `PrimeEntry`, `Prime`.
- `internal/cli/prime.go` — `mtt prime`, `primeJSON`/`toPrimeJSON`, `writePrime`.
- Test files alongside + golden `internal/adapter/yaml/testdata/golden/note_priority.md`.

**Modify:**
- `pkg/mtt/note.go` — add `Priority` to `Note`.
- `internal/adapter/yaml/note_dto.go` — `ymlNote.Priority` + map.
- `internal/core/note.go` — `NoteParams.Priority`, `NoteEditParams.Priority` (+ guard), `NoteFilter{Priorities, Sort}`, `SelectNotes` filter+sort; `NoteAdder`/`NoteEditor` apply priority.
- `internal/cli/note.go` — `--priority` on `note add`/`note edit`; `note list --priority`/`--sort`; `noteJSON.priority`; `writeNote` priority line.
- `internal/cli/root.go` — register `newPrimeCmd()`.
- Docs + `CLAUDE.md` files (Task 6).

---

## Task 1: `Note.Priority` domain field + YAML frontmatter round-trip

**Files:**
- Modify: `pkg/mtt/note.go` (add `Priority Priority`)
- Modify: `internal/adapter/yaml/note_dto.go` (`ymlNote.Priority`, map both ways)
- Create: `internal/adapter/yaml/testdata/golden/note_priority.md`
- Test: `internal/adapter/yaml/note_dto_test.go` (extend)

**Interfaces:**
- Produces: `mtt.Note.Priority mtt.Priority`; frontmatter `priority:` after `tags`, before `refs`.

- [ ] **Step 1: Write the failing test** — append to `internal/adapter/yaml/note_dto_test.go`:

```go
func withPriority(n mtt.Note, p mtt.Priority) mtt.Note { n.Priority = p; return n }

func TestMarshalParseNotePriority(t *testing.T) {
	in := withPriority(fixedNote(), mtt.PriorityHigh)
	data, err := marshalNote(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), "priority: high") {
		t.Fatalf("priority not serialized:\n%s", data)
	}
	got, err := parseNote(in.Slug, data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Priority != mtt.PriorityHigh {
		t.Fatalf("priority round-trip: got %q", got.Priority)
	}
}

func TestMarshalNoteNoPriorityUnchanged(t *testing.T) {
	data, err := marshalNote(fixedNote()) // fixedNote has no priority
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "priority:") {
		t.Fatalf("unset priority must be omitted:\n%s", data)
	}
}
```

Add a `TestNoteGolden` row: `{"priority", withPriority(fixedNote(), mtt.PriorityHigh), "note_priority.md"}`.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/adapter/yaml/ -run 'NotePriority|NoPriorityUnchanged|NoteGolden'`
Expected: FAIL — `mtt.Note` has no field `Priority`.

- [ ] **Step 3: Add the domain field** — in `pkg/mtt/note.go`, add `Priority` after `Tags` (cosmetic; no tags in domain):

```go
type Note struct {
	Slug     NoteSlug
	Title    string
	Tags     []string
	Priority Priority
	Body     string
	Refs     []Ref
	Created  time.Time
	Updated  time.Time
}
```

- [ ] **Step 4: Serialize** — in `internal/adapter/yaml/note_dto.go`, add `Priority` to `ymlNote` (after `Tags`, before `Refs`) and map both ways (plain copy, like `ymlTask.Priority`):

```go
type ymlNote struct {
	Title    string   `yaml:"title,omitempty"`
	Tags     []string `yaml:"tags,omitempty"`
	Priority string   `yaml:"priority,omitempty"`
	Refs     []ymlRef `yaml:"refs,omitempty"`
	Created  string   `yaml:"created"`
	Updated  string   `yaml:"updated"`
}
```

In `marshalNote`'s `ymlNote{...}` literal add `Priority: string(n.Priority)`; in `parseNote`'s returned `mtt.Note{...}` add `Priority: mtt.Priority(yn.Priority)` (a corrupt value round-trips as-is and ranks medium — validity is a CLI concern, D2).

- [ ] **Step 5: Generate the golden + run green**

Run: `go test ./internal/adapter/yaml/ -run TestNoteGolden -update` then inspect `internal/adapter/yaml/testdata/golden/note_priority.md` (expect `priority: high` after `tags`, before nothing/created; timestamps quoted). Then:
Run: `go test ./internal/adapter/yaml/ ./pkg/mtt/`
Expected: PASS (existing `note_min.md`/`note_full.md`/`note_refs.md` byte-identical — `omitempty`).

- [ ] **Step 6: Commit**

```bash
git add pkg/mtt/note.go internal/adapter/yaml/note_dto.go internal/adapter/yaml/note_dto_test.go internal/adapter/yaml/testdata/golden/note_priority.md
git commit -m "t51: Note.Priority domain field + frontmatter round-trip"
```

---

## Task 2: Core — note priority in usecases + filter/sort

**Files:**
- Modify: `internal/core/note.go` (`NoteParams.Priority`, `NoteEditParams.Priority` + guard, `NoteFilter{Priorities, Sort}`, `SelectNotes`, `NoteAdder`/`NoteEditor`)
- Test: `internal/core/note_test.go` (extend)

**Interfaces:**
- Consumes: `SortKey`/`SortPriority`/`lessByRecency` (`list.go`), `anyOrEmpty` (`list.go`), `mtt.Priority.Rank()`.
- Produces:
  - `NoteParams.Priority mtt.Priority`; `NoteEditParams.Priority *mtt.Priority` (nil = unchanged; `&""` clears).
  - `NoteFilter{Tags []string; Priorities []mtt.Priority; Sort SortKey}`.
  - `SelectNotes` filters on `Priorities` (stored-label, `anyOrEmpty`) and, when `Sort == SortPriority`, orders by `Priority.Rank()` asc then recency; else recency (unchanged default).

- [ ] **Step 1: Write the failing test** — append to `internal/core/note_test.go`:

```go
func TestNoteAdderEditorPriority(t *testing.T) {
	kb := newFakeKB()
	got, err := NewNoteAdder(kb, testClock).Add(NoteParams{Slug: "a", Priority: mtt.PriorityHigh})
	if err != nil || got.Priority != mtt.PriorityHigh {
		t.Fatalf("add priority: %+v err=%v", got, err)
	}
	cleared := mtt.Priority("")
	got, err = NewNoteEditor(kb, testClock).Edit("a", NoteEditParams{Priority: &cleared})
	if err != nil || got.Priority != "" {
		t.Fatalf("clear priority: %+v err=%v", got, err)
	}
	// nothing-to-edit guard still fires when no field is provided
	if _, err := NewNoteEditor(kb, testClock).Edit("a", NoteEditParams{}); err == nil {
		t.Fatal("empty edit must error")
	}
}

func TestSelectNotesPriorityFilterSort(t *testing.T) {
	notes := []mtt.Note{
		{Slug: "hi", Priority: mtt.PriorityHigh, Created: time.Unix(10, 0)},
		{Slug: "lo", Priority: mtt.PriorityLow, Created: time.Unix(20, 0)},
		{Slug: "un", Created: time.Unix(30, 0)}, // unset
	}
	// filter: only high (stored-label match)
	f := SelectNotes(notes, NoteFilter{Priorities: []mtt.Priority{mtt.PriorityHigh}})
	if len(f) != 1 || f[0].Slug != "hi" {
		t.Fatalf("priority filter: %+v", f)
	}
	// sort priority: high, then low, then unset (ranks medium -> between)
	s := SelectNotes(notes, NoteFilter{Sort: SortPriority})
	order := []mtt.NoteSlug{s[0].Slug, s[1].Slug, s[2].Slug}
	if order[0] != "hi" || order[2] != "lo" { // high first, low last; unset(medium) middle
		t.Fatalf("priority sort: %+v", order)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/core/ -run 'NoteAdderEditorPriority|SelectNotesPriority'`
Expected: FAIL — `NoteParams`/`NoteEditParams`/`NoteFilter` have no such fields.

- [ ] **Step 3: Implement** — in `internal/core/note.go`:

1. `NoteParams` gains `Priority mtt.Priority`; in `NoteAdder.Add`, set `Priority: p.Priority` in the `mtt.Note{...}` literal.
2. `NoteEditParams` gains `Priority *mtt.Priority`; update the guard and apply:

```go
type NoteEditParams struct {
	Title    *string
	Tags     *[]string
	Body     *string
	Priority *mtt.Priority
}
```
In `NoteEditor.Edit`, change the guard to `if p.Title == nil && p.Tags == nil && p.Body == nil && p.Priority == nil {` and add, beside the other applies, `if p.Priority != nil { n.Priority = *p.Priority }`.

3. `NoteFilter` gains `Priorities []mtt.Priority` and `Sort SortKey`. In `SelectNotes`, add the priority filter to the keep-condition and fold the sort:

```go
func SelectNotes(notes []mtt.Note, f NoteFilter) []mtt.Note {
	out := make([]mtt.Note, 0, len(notes))
	for _, n := range notes {
		if anyOrEmptyIntersect(f.Tags, n.Tags) && anyOrEmpty(f.Priorities, n.Priority) {
			out = append(out, n)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if f.Sort == SortPriority {
			ri, rj := out[i].Priority.Rank(), out[j].Priority.Rank()
			if ri != rj {
				return ri < rj
			}
		}
		return lessNotesByRecency(out[i], out[j])
	})
	return out
}

// lessNotesByRecency is the existing Created-desc, slug-tiebreak order (extracted
// so the priority sort can fall back to it — mirrors lessByPriority/lessByRecency).
func lessNotesByRecency(a, b mtt.Note) bool {
	if !a.Created.Equal(b.Created) {
		return a.Created.After(b.Created)
	}
	return a.Slug < b.Slug
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/core/ -run 'Note'`
Expected: PASS (the sole existing `SelectNotes` caller `NoteFilter{Tags: normTags}` still compiles — named-field literal).

- [ ] **Step 5: Commit**

```bash
git add internal/core/note.go internal/core/note_test.go
git commit -m "t51: core note priority — usecases + filter/sort (mirror task list)"
```

---

## Task 3: Core — `Prime`

**Files:**
- Create: `internal/core/prime.go`
- Test: `internal/core/prime_test.go`

**Interfaces:**
- Consumes: `Backlinks.To(RefNote, slug)` (t1), `mtt.Priority.Rank()`, `lessNotesByRecency` (Task 2).
- Produces:
  - `type PrimeOptions struct { MinPriority mtt.Priority; Limit int }`
  - `type PrimeEntry struct { Slug mtt.NoteSlug; Title string; Tags []string; Priority mtt.Priority; Backlinks int }`
  - `func Prime(notes []mtt.Note, bl Backlinks, opts PrimeOptions) ([]PrimeEntry, int)` — eligible (D7), ordered (priority Rank asc, backlink-count desc, recency), capped; the `int` is `total` (eligible before cap).

- [ ] **Step 1: Write the failing test** — `internal/core/prime_test.go`:

```go
package core

import (
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestPrimeThresholdOrderCap(t *testing.T) {
	notes := []mtt.Note{
		{Slug: "h1", Priority: mtt.PriorityHigh, Created: time.Unix(10, 0)},
		{Slug: "h2", Priority: mtt.PriorityHigh, Created: time.Unix(20, 0)},
		{Slug: "m1", Priority: mtt.PriorityMedium, Created: time.Unix(30, 0)},
		{Slug: "un", Created: time.Unix(40, 0)}, // unset — never primed
	}
	// h1 referenced by two carriers, h2 by none -> h1 first on the backlink tiebreak
	tasks := []mtt.Task{
		{ID: "t1", Refs: []mtt.Ref{{Kind: mtt.RefNote, ID: "h1"}}},
		{ID: "t2", Refs: []mtt.Ref{{Kind: mtt.RefNote, ID: "h1"}}},
	}
	bl := NewBacklinks(tasks, notes)

	// default: high only, uncapped
	got, total := Prime(notes, bl, PrimeOptions{MinPriority: mtt.PriorityHigh, Limit: 0})
	if total != 2 || len(got) != 2 {
		t.Fatalf("high total/shown: total=%d got=%+v", total, got)
	}
	if got[0].Slug != "h1" || got[0].Backlinks != 2 || got[1].Slug != "h2" {
		t.Fatalf("backlink tiebreak: %+v", got)
	}
	// medium threshold: high + medium, still NOT unset
	got, total = Prime(notes, bl, PrimeOptions{MinPriority: mtt.PriorityMedium, Limit: 0})
	if total != 3 {
		t.Fatalf("medium total: %d", total)
	}
	for _, e := range got {
		if e.Slug == "un" {
			t.Fatal("unset note must never be primed")
		}
	}
	// cap: limit 1 over 2 eligible-high -> shown 1, total 2
	got, total = Prime(notes, bl, PrimeOptions{MinPriority: mtt.PriorityHigh, Limit: 1})
	if len(got) != 1 || total != 2 {
		t.Fatalf("cap: shown=%d total=%d", len(got), total)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/core/ -run 'Prime'`
Expected: FAIL — undefined `Prime`.

- [ ] **Step 3: Implement** `internal/core/prime.go`:

```go
package core

import (
	"sort"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// PrimeOptions parameterizes the KB digest. MinPriority is the eligibility
// threshold (default the CLI sets high); Limit caps the shown entries (<=0 = no cap).
type PrimeOptions struct {
	MinPriority mtt.Priority
	Limit       int
}

// PrimeEntry is one important note in the digest (a pointer — no Body).
type PrimeEntry struct {
	Slug      mtt.NoteSlug
	Title     string
	Tags      []string
	Priority  mtt.Priority
	Backlinks int
}

// Prime is the pure derived KB digest (like Roadmap; not in the pkg/mtt contract).
// Eligible ⇔ the note has an EXPLICIT priority (Priority != "") whose Rank() is at
// or above MinPriority; unset notes are NEVER primed (the opt-in safety model). The
// order is priority band (Rank asc), then backlink-count desc, then recency. The
// second return is total — the eligible count BEFORE the Limit cap (the footer's M).
func Prime(notes []mtt.Note, bl Backlinks, opts PrimeOptions) ([]PrimeEntry, int) {
	threshold := opts.MinPriority.Rank()
	type scored struct {
		n  mtt.Note
		bc int
	}
	var elig []scored
	for _, n := range notes {
		if n.Priority == "" || n.Priority.Rank() > threshold {
			continue
		}
		elig = append(elig, scored{n: n, bc: len(bl.To(mtt.RefNote, string(n.Slug)))})
	}
	sort.SliceStable(elig, func(i, j int) bool {
		ri, rj := elig[i].n.Priority.Rank(), elig[j].n.Priority.Rank()
		if ri != rj {
			return ri < rj
		}
		if elig[i].bc != elig[j].bc {
			return elig[i].bc > elig[j].bc // more-referenced first
		}
		return lessNotesByRecency(elig[i].n, elig[j].n)
	})
	total := len(elig)
	if opts.Limit > 0 && len(elig) > opts.Limit {
		elig = elig[:opts.Limit]
	}
	out := make([]PrimeEntry, 0, len(elig))
	for _, s := range elig {
		out = append(out, PrimeEntry{Slug: s.n.Slug, Title: s.n.Title, Tags: s.n.Tags, Priority: s.n.Priority, Backlinks: s.bc})
	}
	return out, total
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/core/ -run 'Prime'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/prime.go internal/core/prime_test.go
git commit -m "t51: core Prime — opt-in threshold, backlink tiebreak, cap + total"
```

---

## Task 4: CLI — note `--priority` (add/edit) + `note list --priority`/`--sort`

**Files:**
- Modify: `internal/cli/note.go` (flags + wiring; `noteJSON.priority`; `writeNote` priority line)
- Test: `internal/cli/testdata/scripts/note_priority.txt`

**Interfaces:**
- Consumes: `parsePriority`/`toPriorities` (`priority.go`), `core.NoteParams.Priority`/`NoteEditParams.Priority`/`NoteFilter{Priorities, Sort}` (Task 2), `core.SortKey`/`SortPriority`.

- [ ] **Step 1: Write the e2e first** `internal/cli/testdata/scripts/note_priority.txt`: `note add a --priority high` → `note show a` shows `priority: high`; `note add b` (unset); `note list --priority high` lists only `a`; `note list --sort priority` orders `a` before `b`; `note edit a --priority ""` clears (show no priority line); an invalid `--priority bogus` on add → exit 1. Mirror `note.txt`.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/cli/ -run 'TestScripts/note_priority'`
Expected: FAIL — `--priority`/`--sort` are unknown flags.

- [ ] **Step 3: Implement** — in `internal/cli/note.go`:

1. `noteJSON` gains `Priority string \`json:"priority,omitempty"\``; in `toNoteJSON`, `Priority: string(n.Priority)`.
2. `writeNote`: after the `tags` line, add (when set):

```go
if n.Priority != "" {
	fmt.Fprintf(&b, "  priority: %s\n", n.Priority)
}
```

3. `newNoteAddCmd`: add `var priority string`; flag `cmd.Flags().StringVar(&priority, "priority", "", "note priority: high|medium|low")`; parse `pr, err := parsePriority(priority)` (before the store call; usage error on bad value) and pass `Priority: pr` in `core.NoteParams{...}`.
4. `newNoteEditCmd`: add `var priority string`; flag `cmd.Flags().StringVar(&priority, "priority", "", "new priority: high|medium|low (empty string clears it)")`; and (mirroring task `edit`):

```go
if cmd.Flags().Changed("priority") {
	pr, err := parsePriority(priority)
	if err != nil {
		return err
	}
	p.Priority = &pr
}
```

5. `newNoteListCmd`: add `--priority` (repeatable) + `--sort`:

```go
var priorities []string
var sortKey string
// in RunE, validate sort (like cli/list.go):
switch sortKey {
case "", string(core.SortCreated), string(core.SortUpdated), string(core.SortPriority):
default:
	return fmt.Errorf("invalid --sort %q: want created|updated|priority", sortKey)
}
prios, err := toPriorities(priorities)
if err != nil {
	return err
}
sel := core.SelectNotes(notes, core.NoteFilter{Tags: normTags, Priorities: prios, Sort: core.SortKey(sortKey)})
// flags:
cmd.Flags().StringArrayVar(&priorities, "priority", nil, "filter by priority: high|medium|low (repeatable)")
cmd.Flags().StringVar(&sortKey, "sort", "", "sort order: created|updated|priority (default created)")
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/cli/ -run 'TestScripts/note_priority'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/note.go internal/cli/testdata/scripts/note_priority.txt
git commit -m "t51: note --priority (add/edit) + note list --priority/--sort"
```

---

## Task 5: CLI — `mtt prime`

**Files:**
- Create: `internal/cli/prime.go`
- Modify: `internal/cli/root.go` (register `newPrimeCmd()`)
- Test: `internal/cli/testdata/scripts/prime.txt`

**Interfaces:**
- Consumes: `core.Prime`/`PrimeOptions`/`PrimeEntry` (Task 3), `core.NewBacklinks` (t1), `parsePriority`, `mtt.PriorityHigh`.
- Produces: `mtt prime [--min-priority] [--limit] [--json]`; `primeJSON`/`toPrimeJSON`; `writePrime`.

- [ ] **Step 1: Write the e2e first** `internal/cli/testdata/scripts/prime.txt`: empty repo → `mtt prime` prints the "no important notes" line (exit 0); `note add a --priority high` + `note add b --priority medium` + `note add c` → `mtt prime` shows only `a` with footer `1 of 1`; `mtt prime --min-priority medium` shows `a` and `b`, footer `2 of 2` (not `c`); `mtt prime --min-priority medium --limit 1` shows one with footer `1 of 2`; `mtt prime --json` emits `"priority": "high"` and a non-null array. Assert bodies never appear.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/cli/ -run 'TestScripts/prime'`
Expected: FAIL — unknown command `prime`.

- [ ] **Step 3: Implement** `internal/cli/prime.go`:

```go
package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newPrimeCmd builds `mtt prime`: a curated, opt-in, pointer-only KB digest for
// session-start injection. A pure read; no mutation.
func newPrimeCmd() *cobra.Command {
	var minPriority string
	var limit int
	cmd := &cobra.Command{
		Use:   "prime",
		Short: "Print a curated digest of the important KB notes (for session start)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			min := mtt.Priority(minPriority)
			if !min.Valid() || min == "" {
				return fmt.Errorf("invalid --min-priority %q: want high|medium|low", minPriority)
			}
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			notes, err := yaml.NewKnowledgeStore(root).ListNotes()
			if err != nil {
				return err
			}
			tasks, err := yaml.NewTaskStore(root).List()
			if err != nil {
				return err
			}
			entries, total := core.Prime(notes, core.NewBacklinks(tasks, notes), core.PrimeOptions{MinPriority: min, Limit: limit})
			if jsonFlag(cmd) {
				out := make([]primeJSON, 0, len(entries))
				for _, e := range entries {
					out = append(out, toPrimeJSON(e))
				}
				return writeJSON(cmd.OutOrStdout(), out)
			}
			return writePrime(cmd.OutOrStdout(), entries, total)
		},
	}
	cmd.Flags().StringVar(&minPriority, "min-priority", "high", "include notes at or above this priority: high|medium|low")
	cmd.Flags().IntVar(&limit, "limit", 20, "cap the digest to N notes (<=0 = no cap)")
	return cmd
}

// primeJSON is the machine view of one digest entry (tags forced non-null — the
// toNoteJSON house rule). backlinks is the incoming-reference count.
type primeJSON struct {
	Slug      string   `json:"slug"`
	Title     string   `json:"title,omitempty"`
	Tags      []string `json:"tags"`
	Priority  string   `json:"priority"`
	Backlinks int      `json:"backlinks"`
}

func toPrimeJSON(e core.PrimeEntry) primeJSON {
	tags := e.Tags
	if tags == nil {
		tags = []string{}
	}
	return primeJSON{Slug: string(e.Slug), Title: e.Title, Tags: tags, Priority: string(e.Priority), Backlinks: e.Backlinks}
}

// writePrime renders the markdown digest (D4): a header, one pointer line per entry,
// and the "N of M" footer. An empty digest prints a single actionable line.
func writePrime(w io.Writer, entries []core.PrimeEntry, total int) error {
	var b strings.Builder
	if len(entries) == 0 {
		fmt.Fprintln(&b, "# Knowledge base — no important notes (mark one: mtt note edit <slug> --priority high)")
		_, err := fmt.Fprint(w, b.String())
		return err
	}
	b.WriteString("# Knowledge base — important notes\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "- **%s**  [%s]", e.Slug, e.Priority)
		if len(e.Tags) > 0 {
			fmt.Fprintf(&b, "  (%s)", strings.Join(e.Tags, ", "))
		}
		if e.Title != "" {
			fmt.Fprintf(&b, "  — %s", e.Title)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "(%d of %d important notes shown — `mtt note show <slug>` for detail)\n", len(entries), total)
	_, err := fmt.Fprint(w, b.String())
	return err
}
```

Register in `internal/cli/root.go`: add `newPrimeCmd()` to the `AddCommand(...)` list.

- [ ] **Step 4: Run to verify it passes**

Run: `make check`
Expected: PASS (full gate — new command + core; race + lint + build).

- [ ] **Step 5: Commit**

```bash
git add internal/cli/prime.go internal/cli/root.go internal/cli/testdata/scripts/prime.txt
git commit -m "t51: mtt prime — curated pointer-only KB digest (markdown + --json)"
```

---

## Task 6: Docs sync

**Files (all Modify):** `CLI_REFERENCE.md` ↔ `.ru.md`; `DESIGN.md` ↔ `.ru.md`; `docs/architecture/model.go`; `pkg/mtt/CLAUDE.md`, `internal/core/CLAUDE.md`, `internal/cli/CLAUDE.md`, `internal/adapter/yaml/CLAUDE.md`.

**No test cycle** (docs). Grep every parallel occurrence (EN + RU) before editing.

- [ ] **Step 1:** `CLI_REFERENCE.md` — add an **`mtt prime`** section (surface, `--min-priority`/`--limit`/`--json` defaults, the markdown format + `N of M` footer, the opt-in threshold: unset never primed); add `--priority` to `note add`/`note edit` and `--priority`/`--sort` to `note list`; add a **`sessionStart` hook snippet** (a `settings.json` example running `mtt prime`). Mirror into `CLI_REFERENCE.ru.md`.

- [ ] **Step 2:** `DESIGN.md` — a "**Shipped (t51)**" note under the KB section (note-importance axis via `Note.Priority`; `mtt prime` = curated, opt-in, pointer-only digest; ranking = priority + backlink tiebreak; the hook is config, not code; KB stays a supporting feature). Mirror into `DESIGN.ru.md`.

- [ ] **Step 3:** `docs/architecture/model.go` — add `Priority` to the `Note` block; note `Prime`/`PrimeEntry` are core-derived (not contract), like `Roadmap`.

- [ ] **Step 4:** CLAUDE.md — `pkg/mtt` (`Note.Priority`), `internal/core` (note priority in `NoteParams`/`NoteEditParams`/`NoteFilter`+`SelectNotes` sort; `Prime`), `internal/cli` (`mtt prime`, note `--priority`/list sort-filter), `internal/adapter/yaml` (note frontmatter `priority`). Keep each thin.

- [ ] **Step 5: Verify + commit**

```bash
make check
git add -A
git commit -m "t51: docs sync — CLI_REFERENCE/DESIGN (EN+RU), model.go, CLAUDE.md files"
```

---

## Self-review (completed before submit)

- **Spec coverage:** D1 ranking → T1,T2,T3; D2 Note.Priority → T1; D3 pointer-only → T3,T5; D4 command/format/footer → T5; D5 core.Prime → T3; D6 note CLI → T2,T4; D7 threshold/unset/corrupt → T3; D8 hook (docs) → T6; D9 KB-only → T5 (prime prints no tasks). All covered.
- **Type consistency:** `PrimeOptions`/`PrimeEntry`/`Prime(...)([]PrimeEntry,int)`, `NoteFilter{Priorities,Sort}`, `NoteEditParams.Priority *mtt.Priority`, `primeJSON`/`toPrimeJSON`, `lessNotesByRecency` — used consistently T1–T5. `SortPriority` is the reused `SortKey` constant (no standalone `SortNotes*`).
- **Build-green per task:** every change is **additive** (new fields on `NoteFilter`/`NoteParams`/`NoteEditParams` — named-field literals unaffected; new files; new flags) — **no signature-break ripple**. Each task's `make check` is green.
- **Placeholder scan:** none — every code step has real code.
- **Confirmed against code:** `Priority` VO + `PriorityHigh/Medium/Low` + `Rank()` (`pkg/mtt/priority.go`); `ListFilter.Sort`+`lessByPriority`+`anyOrEmpty` (`internal/core/list.go`); task `edit --priority` precedent (`*mtt.Priority`+`Changed`+guard, `internal/core/edit.go`/`internal/cli/edit.go`); `parsePriority`/`toPriorities` (`internal/cli/priority.go`); `Backlinks.To` (`internal/core/backlinks.go`); `roadmapJSON`/`toRoadmapJSON`/`writeRoadmap` render pattern (`internal/cli/roadmap.go`); `SelectNotes`/`NoteFilter`/`NoteParams`/`NoteEditParams`/`noteJSON`/`writeNote`/`newNoteListCmd` (`internal/core/note.go`, `internal/cli/note.go`); note goldens live under `testdata/golden/`.
