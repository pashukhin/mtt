# tool-wide --tag comma-split (t50) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `--tag`/`--exclude-tag` accept comma-separated values (`--tag a,b,c`) like `--depends-on`, tool-wide, staying repeatable; and `mtt add`'s too-many-arguments error names the real cause (quote the title / comma-or-repeat the flag).

**Architecture:** CLI-only, mechanical. Convert every `--tag`/`--exclude-tag` flag from cobra `StringArray` to `StringSlice` (comma-split + repeatable) across the 13 registration sites + the one selector getter; rewrite one error string. No `core`/domain/store/`toTags`/`NormalizeTag` change.

**Tech Stack:** Go 1.23+, cobra/pflag (`StringSliceVar`/`GetStringSlice`), `testscript` e2e.

## Global Constraints

- **Spec of record:** `docs/superpowers/specs/t50-tag-comma-split.md` (D1–D3, AC1–AC7).
- **TDD:** e2e-first (red on a comma value → green after conversion). `make check` green before every commit.
- **Touch ONLY the tag flags.** Do **not** convert the non-tag `StringArray` flags in these files: `--ref`
  (`add.go:97`, `note.go:137`), `--status`/`--type`/`--kind`/`--priority` (list/tree/ready/tags/selector),
  `note list --priority` (`note.go:187`). They are out of scope (D3).
- **selector.go is the silent-failure site:** its registration (`StringArray`→`StringSlice`) **and** getter
  (`GetStringArray`→`GetStringSlice`) MUST change together, or the `_`-discarded getter error drops the filter
  silently (AC2a guards this).
- **Back-compat:** `--tag a --tag b` (repeat) still works; `--tag a,b` (was a usage error) now splits; `--tag ""`
  now collapses to `[]` (the one documented shift — AC6).

---

## File structure

**Modify (flag conversions — 14 edits):**
- `internal/cli/add.go:96` (`--tag`)
- `internal/cli/note.go:133` (`note add --tag`), `:186` (`note list --tag`), `:348` (`note edit --tag`)
- `internal/cli/list.go:96` (`--tag`), `:97` (`--exclude-tag`)
- `internal/cli/tree.go:87` (`--tag`), `:88` (`--exclude-tag`)
- `internal/cli/ready.go:78` (`--tag`), `:79` (`--exclude-tag`)
- `internal/cli/tags.go:90` (`--tag`), `:91` (`--exclude-tag`)
- `internal/cli/selector.go:28` (registration), `:51` (getter)

**Modify (error message):** `internal/cli/add.go:39`.

**Create:** `internal/cli/testdata/scripts/tag_comma.txt` (e2e for AC1–AC6).

**Docs:** `CLI_REFERENCE.md`↔`.ru.md`, `CHANGELOG.md`, `internal/cli/CLAUDE.md`.

---

## Task 1: `--tag`/`--exclude-tag` comma-split (e2e-first, red→green)

**Files:** the 14 conversion edits above + `internal/cli/testdata/scripts/tag_comma.txt`

- [ ] **Step 1: Write the failing e2e** — `internal/cli/testdata/scripts/tag_comma.txt`:

```
# t50 — --tag / --exclude-tag accept comma-separated values (like --depends-on), tool-wide.
exec mtt init
cp flow.yaml .mtt/config.yaml

# AC1: comma-split authoring.
exec mtt add 'x' --tag a,b,c
exec mtt show t1 --json
stdout '"a"'
stdout '"b"'
stdout '"c"'
# AC1: comma + repeat compose.
exec mtt add 'y' --tag a,b --tag c
exec mtt show t2 --json
stdout '"c"'

# AC3: the repeated-only form still works (regression).
exec mtt add 'z' --tag a --tag b
exec mtt show t3 --json
stdout '"a"'
stdout '"b"'

# AC2: comma-split filtering on list (OR-within) + exclude.
exec mtt list --tag a,c --ids
stdout 't1'
exec mtt list --exclude-tag a,b,c --ids
! stdout 't1'

# AC2a: the selector path (the silent-failure guard) — tag add via a --tag selector.
exec mtt tag add urgent --tag a,b
exec mtt list --tag urgent --ids
stdout 't1'

# AC5: a malformed element and a trailing comma both stay clean usage errors.
! exec mtt add 'bad' --tag 'a,b!'
! exec mtt add 'bad2' --tag 'a,'

# AC6: empty value — add is a no-op (exit 0, no tags), not the old error.
exec mtt add 'empty' --tag ''
exec mtt show t4 --json
! stdout '"tags"'

-- flow.yaml --
version: 1
project: {name: tagtest}
types:
  - name: task
    prefix: t
    default: true
    statuses:
      - {name: tbd, kind: initial, default: true}
      - {name: in_progress, kind: active}
      - {name: done, kind: terminal}
    transitions:
      - {from: tbd, to: in_progress}
      - {from: in_progress, to: done}
```

- [ ] **Step 2: Run it to verify it fails (RED)**

Run: `go test ./internal/cli/ -run 'TestScripts/tag_comma' -count=1`
Expected: FAIL at the first `mtt add 'x' --tag a,b,c` — today `StringArray` passes `"a,b,c"` as one value,
`toTags`→`NormalizeTag("a,b,c")` rejects it (comma outside the charset) → a usage error, so the command exits
non-zero and `mtt show` never shows `a`/`b`/`c`.

- [ ] **Step 3: Convert the authoring flags** — `add.go:96` and `note.go:133/186/348`:

`internal/cli/add.go:96`
```go
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "add a tag (repeatable, comma-separated; #hashtags in the title/description are also picked up)")
```
`internal/cli/note.go:133`
```go
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "add a tag (repeatable, comma-separated)")
```
`internal/cli/note.go:186`
```go
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "filter by tag (repeatable, comma-separated; OR within)")
```
`internal/cli/note.go:348`
```go
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "replace the tag set (repeatable, comma-separated)")
```

- [ ] **Step 4: Convert the filter flags** — `list.go`, `tree.go`, `ready.go`, `tags.go` (each has a `--tag`
and an `--exclude-tag` line; convert BOTH, leave the sibling `--status`/`--type`/`--kind`/`--priority` lines as
`StringArrayVar`):

For each of `list.go:96`, `tree.go:87`, `ready.go:78`, `tags.go:90`:
```go
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "filter by tag (repeatable, comma-separated)")
```
(`note.go:186` differs — done in Step 3.)

For each of `list.go:97`, `tree.go:88`, `ready.go:79`, `tags.go:91`:
```go
	cmd.Flags().StringSliceVar(&excludeTags, "exclude-tag", nil, "exclude tasks carrying this tag (repeatable, comma-separated)")
```

- [ ] **Step 5: Convert the selector (registration AND getter — both, or it fails silently)**

`internal/cli/selector.go:28`
```go
	cmd.Flags().StringSlice("tag", nil, "select by tag (repeatable, comma-separated)")
```
`internal/cli/selector.go:51`
```go
	tags, _ := cmd.Flags().GetStringSlice("tag")
```
(Leave `selector.go:24-27` and `:47-50` — the non-tag flags — as `StringArray`/`GetStringArray`.)

- [ ] **Step 6: Run the e2e to verify it passes (GREEN)**

Run: `go test ./internal/cli/ -run 'TestScripts/tag_comma' -race -count=1`
Expected: PASS (AC1/AC2/AC2a/AC3/AC5/AC6 all satisfied). The AC2a `tag add --tag a,b` line proves the selector
getter was converted (a half-done conversion would drop the filter → `urgent` applied to nothing → `mtt list
--tag urgent --ids` empty → FAIL).

- [ ] **Step 7: No-regression on the existing tag tests + `make check`**

Run: `go test ./internal/cli/ -run 'TestScripts' -race -count=1` (the existing `tags`/`list`/`selector`
scripts stay green — repeated `--tag a --tag b` unchanged).
Run: `make check`
Expected: green.

- [ ] **Step 8: Commit**

```bash
git add internal/cli/add.go internal/cli/note.go internal/cli/list.go internal/cli/tree.go \
        internal/cli/ready.go internal/cli/tags.go internal/cli/selector.go \
        internal/cli/testdata/scripts/tag_comma.txt
git commit -m "t50: --tag/--exclude-tag accept comma-separated values (StringSlice), tool-wide"
```

---

## Task 2: clearer too-many-positionals error on `mtt add` (D2)

**Files:** `internal/cli/add.go:39`; extend `internal/cli/testdata/scripts/tag_comma.txt`

- [ ] **Step 1: Add the failing e2e (AC4)** — append to `tag_comma.txt` (before the `-- flow.yaml --` line):

```
# AC4: extra positionals -> the message names both remedies (quote title / comma-or-repeat the flag).
! exec mtt add 'fix login' extra
stderr 'too many arguments'
stderr 'comma-separated'
stderr 'repeating the flag'
```

- [ ] **Step 2: Run to verify it fails (RED)**

Run: `go test ./internal/cli/ -run 'TestScripts/tag_comma' -count=1`
Expected: FAIL — the current message is `too many arguments: wrap a multi-word title in quotes …`, which lacks
`comma-separated`/`repeating the flag`.

- [ ] **Step 3: Rewrite the message** — `internal/cli/add.go` Args func (the `len(args) > 1` branch, ~L38-39):

```go
			if len(args) > 1 {
				return fmt.Errorf("too many arguments (got %d): wrap a multi-word title in quotes (mtt add \"fix login\"), and pass multiple --tag/--depends-on values comma-separated (--tag a,b) or by repeating the flag (--tag a --tag b) — not space-separated", len(args))
			}
```
**Import fix (verified):** `errors` is used **only** on this line in `add.go` — switching to `fmt.Errorf`
(`fmt` already imported) leaves `errors` unused, which fails the build. **Remove the `"errors"` import line**
from `add.go`'s import block in the same edit.

- [ ] **Step 4: Run to verify it passes (GREEN)**

Run: `go test ./internal/cli/ -run 'TestScripts/tag_comma' -race -count=1`
Expected: PASS.

- [ ] **Step 5: `make check` + commit**

Run: `make check`
```bash
git add internal/cli/add.go internal/cli/testdata/scripts/tag_comma.txt
git commit -m "t50: mtt add too-many-arguments error names the comma/repeat-the-flag remedy"
```

---

## Task 3: docs sync

**Files:** `CLI_REFERENCE.md`↔`.ru.md`, `CHANGELOG.md`, `internal/cli/CLAUDE.md`

- [ ] **Step 1: `CLI_REFERENCE.md` + `.ru.md`** — grep the tag flags: `grep -n '\-\-tag\|--exclude-tag'
CLI_REFERENCE.md CLI_REFERENCE.ru.md`. Where `--tag`/`--exclude-tag` are described, note they are
**comma-separated or repeatable** (like `--depends-on`). Update EN and the parallel RU line together.

- [ ] **Step 2: `CHANGELOG.md`** — under `[Unreleased] → ### Changed`, append:
```markdown
- **`--tag`/`--exclude-tag` accept comma-separated values** (`--tag a,b,c`) like `--depends-on`, tool-wide
  (authoring + filters), still repeatable. `mtt add`'s too-many-arguments error now explains the comma/repeat
  form instead of only blaming the title.
```

- [ ] **Step 3: `internal/cli/CLAUDE.md`** — in the Tags paragraph, change the `--tag` flag description note:
`--tag`/`--exclude-tag` are now `StringSlice` (comma-split + repeatable), not `StringArray`. One clause; grep
for `StringArrayVar` in the CLAUDE.md prose if mentioned, else add a short note in the Tags section.

- [ ] **Step 4: `make check` + commit**

Run: `make check`
```bash
git add CLI_REFERENCE.md CLI_REFERENCE.ru.md CHANGELOG.md internal/cli/CLAUDE.md
git commit -m "t50: docs — --tag comma-separated (CLI_REFERENCE EN/RU, CHANGELOG, CLAUDE)"
```

---

## Acceptance criteria mapping (spec → tasks)

- **AC1** (comma authoring), **AC2** (comma filtering), **AC2a** (selector guard), **AC3** (repeat regression),
  **AC5** (invalid/trailing-comma still errors), **AC6** (empty-value shift) → Task 1.
- **AC4** (clearer positional error) → Task 2.
- **AC7** (`make check` + docs) → each task's commit + Task 3.

## impl_review checklist

- Principles self-check (KISS; CLI-only, no core change); docs-sync judgment (CLI_REFERENCE EN+RU, CHANGELOG,
  CLAUDE); `make check` green.
- Sanity-run the real binary: `mtt add x --tag a,b,c` (→ 3 tags), `mtt list --tag a,b`, `mtt add "t" extra`
  (→ the new error), `mtt tag add urgent --tag a,b` (selector path).
