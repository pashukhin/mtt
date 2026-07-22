# tool-wide `--tag` ergonomics: comma-split + clearer positional error (`t50`)

Status: spec (decision record). Type: task (`t50`). Branch: `task/t50`. Tags: `ux`, `release`.

## Context / problem

`--depends-on` is a cobra **`StringSlice`** flag, so `mtt add x --depends-on a,b,c` splits on commas into three
ids. `--tag` (and `--exclude-tag`) are **`StringArray`** flags, which do **not** split — `mtt add x --tag a,b,c`
passes the single value `"a,b,c"` to `toTags` → `NormalizeTag("a,b,c")` fails (a comma is outside the tag
charset `\pL\pN\pM._-`) → a usage error. So multi-tag authoring/filtering is inconsistent with `--depends-on`
and forces the verbose repeated form `--tag a --tag b --tag c`.

Worse, a user who reaches for the natural **space-separated** form — `mtt add "fix login" --tag a b c` — hits a
**misleading** error: `StringArray` consumes only `a`, and `b`/`c` fall through as **positionals**, so
`add`'s arg check reports `too many arguments: wrap a multi-word title in quotes` — blaming the (correctly
quoted) title, never hinting that the extra args came from the flag. (Space-separated flag values are
**unfixable** — the shell/cobra can't know `b c` belong to `--tag` — so the fix is to make comma-separated work
*and* to make the error name the real cause.)

Constraints:

- **CLI-only.** No `core`/domain/store/`toTags`/`NormalizeTag` change; this is flag registration + one error
  message. The compat surface (SemVer) touched is the CLI flag grammar — a **backward-compatible** widening.
- **Back-compat.** `--tag a --tag b` (repeated) must keep working; `--tag a,b` was previously a *usage error*
  (never a valid single tag, since a comma can't be in a tag), so making it split is a pure improvement that
  breaks nothing real.
- **TDD, KISS.** e2e-first; no new abstraction.

## Decisions

### D1 — `--tag`/`--exclude-tag` become `StringSlice` everywhere (comma-split + repeatable)

Convert **every** `--tag`/`--exclude-tag` registration from `StringArrayVar`/`StringArray` to
`StringSliceVar`/`StringSlice` (and the one `selector.go` reader from `GetStringArray` to `GetStringSlice`).
The 13 sites:

- **authoring:** `add.go:96` (`--tag`), `note.go:133` (`note add --tag`), `note.go:348` (`note edit --tag`);
- **filtering:** `list.go:96/97`, `tree.go:87/88`, `ready.go:78/79`, `tags.go:90/91`, `note.go:186`
  (`note list --tag`), and `selector.go:28`+`:51` (the `--filter` selector's `tag`).

`StringSlice` splits on commas (cobra uses `encoding/csv` per value) **and** stays repeatable, so all three
forms work and compose: `--tag a,b`, `--tag a --tag b`, `--tag a,b --tag c` → the union. `toTags`/`NormalizeTag`
are unchanged — they still normalize/validate each element and reject a genuinely malformed tag. Flag help
strings gain "comma-separated" (mirroring `--depends-on`'s "repeatable, comma-separated").

- **Rejected — only convert the authoring flags** (`add`/`note add`): leaves `list --tag a,b` still broken and
  the tool inconsistent; the task is explicitly *tool-wide*. Converting all keeps one mental model.
- **CSV caveat (documented, not worked around):** `StringSlice` parses each value as a CSV record, so a value
  containing a comma or quote is special. Tags **cannot** contain a comma (charset), so this never bites a real
  tag; noted so nobody re-litigates it.

### D2 — `mtt add`'s too-many-positionals error names the real cause

Rewrite the message at `add.go:38-39` so it covers **both** ways a user lands there — an unquoted multi-word
title **and** space-separated flag values:

```
too many arguments (got N): wrap a multi-word title in quotes (mtt add "fix login"), and pass multiple
--tag/--depends-on values comma-separated (--tag a,b) or by repeating the flag (--tag a --tag b) — not
space-separated
```

- **`note add` stays on `oneID`** (`note.go:92`): its message already reads `provide exactly one slug (example:
  mtt note add auth-design)` — clear about the single positional, and `oneID` is **shared** by
  `note show/edit/rm`, so widening it risks unrelated wording. Recorded decision: `note add`'s clarity comes
  from D1 (comma-split now works) + the existing `oneID`; no `oneID` change.

## Scope

**In:** the 13 flag conversions (`StringArray`→`StringSlice`, `GetStringArray`→`GetStringSlice`) + help-text
tweak; the `add` error message; e2e coverage; docs sync (CLI_REFERENCE EN/RU, CHANGELOG).

**Out:**
- **`oneID` rewording** — `note add`/`show`/`edit`/`rm` keep the shared message (D2).
- **Space-separated flag values** — unfixable by design (the shell splits them into positionals); D2 only
  makes the *error* actionable.
- **Any `core`/domain/store change** — none; `toTags`/`NormalizeTag` untouched.

## Acceptance criteria

1. **Comma-split authoring (e2e).** `mtt add "x" --tag a,b,c` creates a task with tags `a b c` (asserted via
   `mtt show`/`--json`); `mtt add "y" --tag a,b --tag c` yields `a b c` (comma + repeat compose).
2. **Comma-split filtering (e2e).** With tasks tagged variously, `mtt list --tag a,b` matches tasks carrying
   `a` **or** `b` (OR-within, unchanged semantics — just reachable via one flag now); `mtt list --exclude-tag
   a,b` excludes either. One filter site beyond `list` (e.g. `mtt tags --tag …` or `ready --tag …`) confirms the
   tool-wide conversion.
3. **Repeated form still works (e2e/regression).** `--tag a --tag b` on `add` and a filter still produce the
   union — no regression from the `StringArray`→`StringSlice` switch.
4. **Clearer positional error (e2e).** `mtt add "fix login" extra` (or `--tag a b` where `b` becomes a
   positional) exits non-zero with the new message naming both the quote-the-title and comma/repeat-the-flag
   remedies.
5. **Invalid tag still rejected (e2e).** A malformed element (e.g. `--tag "a,b!"` → the `b!` element) still
   yields the `toTags` usage error — validation is unchanged, only splitting is added.
6. `make check` green. Docs synced (below).

## Testing approach

- **e2e (testscript, hermetic):** extend a tags script (or add `tag_comma.txt`) — AC1–AC5. Assert via
  `mtt show --json` / `mtt list --ids`. No network.
- **Unit:** none new required (`toTags`/`NormalizeTag` unchanged and already covered); the conversion is flag
  wiring, exercised end-to-end by the e2e.

## Docs to sync (docs-sync judgment, `impl_review`)

Grep **all** parallel occurrences (EN + RU) before editing.

- **`CLI_REFERENCE.md ↔ .ru.md`:** where `--tag`/`--exclude-tag` are documented, note they are
  **comma-separated or repeatable** (like `--depends-on`). Grep for `--tag`.
- **`CHANGELOG.md`** `[Unreleased]` → **Changed:** `--tag`/`--exclude-tag` now accept comma-separated values
  (`--tag a,b,c`) like `--depends-on`, and `mtt add`'s too-many-arguments error explains the comma/repeat form.
- **CLAUDE.md:** `internal/cli` already documents the tag flags; add a clause that `--tag`/`--exclude-tag` are
  `StringSlice` (comma-split) — a one-liner. No `core`/`pkg` doc change (unchanged).
- **`AGENTS.md`:** no rule change.

## Sequencing & tracking (process, not code)

`t50` is `speccing` on `task/t50`. This document is the `speccing` deliverable. Next: commit it, adversarial
subagent **spec review**, `spec_human_review` → `planning` → `plan_review` → `plan_human_review` → TDD
`implementing` → `impl_review` → `approved` (auto PR) → merge → `deliver`. Added to the **v0.10.0** batch
(with `t44`/`t14`/`t28`/`t16`) — a cheap user-visible ergonomics win before the cut.
