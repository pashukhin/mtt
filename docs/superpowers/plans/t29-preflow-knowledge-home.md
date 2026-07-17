# t29 — pre-flow knowledge home Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a resumption pointer (`mtt use` / `mtt show`) to the root-help `Long`, so the in-tool orientation covers a *returning* agent, not just a fresh start — the only code change from t29's decision record.

**Architecture:** A one-sentence-clause edit to the root command's `Long` string in `internal/cli/root.go`, plus a help-text test. No new command, no behavior change.

**Tech Stack:** Go (cobra CLI), `internal/cli/root_test.go` (help-text assertion).

## Global Constraints

- Spec (authority): `docs/superpowers/specs/t29-preflow-knowledge-home.md` — decisions D1–D3 binding.
- **No `mtt guide`/`resume` command** (D3); no other command's behavior changes.
- TDD; `make check` green before commit.
- The help-text test must key on a token **unique to the resumption clause** — `resuming` and/or `mtt show` (NOT `mtt use`, which already appears in the `Long`'s Shorthand sentence) — per spec AC-2.

---

### Task 1: Root-help resumption clause + test

**Files:**
- Modify: `internal/cli/root.go:26-27` (the `Long` string's "Start with …" sentence)
- Test: `internal/cli/root_test.go` (new `TestRootHelpMentionsResume`)

**Interfaces:**
- Consumes: `NewRootCmd()` (existing). No new symbols.

- [ ] **Step 1: Write the failing test.** Append to `internal/cli/root_test.go` (it already imports `bytes`/`strings`; mirrors the existing `TestRootHelpMentionsSugar`):

```go
func TestRootHelpMentionsResume(t *testing.T) {
	// t29: the resumption path (mtt use / mtt show) must be discoverable from
	// --help — the in-tool orientation covers a returning agent, not just a
	// fresh start. Key on tokens unique to the resumption clause (NOT "mtt use",
	// which already appears in the Long's Shorthand sentence).
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute --help: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "resuming") || !strings.Contains(got, "mtt show") {
		t.Fatalf("root help must name the resumption path (resuming / mtt show): %s", got)
	}
}
```

- [ ] **Step 2: Run it, verify it fails.**

Run: `go test ./internal/cli/ -run TestRootHelpMentionsResume`
Expected: FAIL — the current `Long` has neither `resuming` nor `mtt show`.

- [ ] **Step 3: Add the resumption clause to the `Long`.** In `internal/cli/root.go`, replace:

```go
(what to do, in order), 'mtt ready' (what is unblocked), 'mtt types' (the flows
and their gates). All commands support --json.`,
```

with:

```go
(what to do, in order), 'mtt ready' (what is unblocked), 'mtt types' (the flows
and their gates); resuming, 'mtt use' shows your current task and 'mtt show' its
status + next moves. All commands support --json.`,
```

- [ ] **Step 4: Run the test, verify it passes.**

Run: `go test ./internal/cli/ -run 'TestRootHelp|TestRootShort' -v`
Expected: PASS — `TestRootHelpMentionsResume`, and the pre-existing `TestRootHelpMentionsSugar` / `TestRootShortNamesTheGate` still pass (the edit keeps `<status>` and the `Short`).

- [ ] **Step 5: Gate.**

Run: `make check`
Expected: `OK: make check passed`.

- [ ] **Step 6: Commit.**

```bash
git add internal/cli/root.go internal/cli/root_test.go
git commit -m "t29: root-help names the resumption path (mtt use / mtt show)"
```

---

## Final acceptance (after the task)

- [ ] **AC-1/2:** `mtt --help` shows the resumption clause (`resuming, 'mtt use' … 'mtt show' …`) alongside the navigation pointers; `TestRootHelpMentionsResume` pins it (keyed on `resuming`/`mtt show`).
- [ ] **AC-3:** no `guide`/`resume` command added (`git diff main...task/t29 --stat` touches only `root.go`, `root_test.go`, and the spec/plan docs); no other command changed.
- [ ] **AC-4:** `make check` green.

## Self-review notes

- **Spec coverage:** D2 → Task 1 (the clause + test). D1/D3 are decisions (no code); the README/runbook homes are t42/t23 (out of scope, per the spec). Testing approach → Step 1 (keyed on `resuming`/`mtt show`, not `mtt use`).
- **No placeholder / no new symbol.** The edit is verbatim; the test mirrors `TestRootHelpMentionsSugar`.
