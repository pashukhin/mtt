# t52 — Scaffold agent settings + hooks — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for
> tracking. The whole implementation lands inside the mtt `implementing` status (one `mtt submit` at the end).

**Goal:** `mtt init` (by default) and a new `mtt agent hooks` command scaffold a Claude Code
`.claude/settings.json` — `SessionStart`+`PreCompact` hooks running `mtt prime` plus a read-only
`permissions.allow` — via an additive, idempotent, no-clobber merge.

**Architecture:** A new `internal/scaffold` package owns a `Harness` seam (interface + registry; only `claude`
registered) whose pure `Merge` computes the desired settings bytes from the current ones; a thin `Run` does the
IO. Durability is shared via a new `internal/fsutil.AtomicWrite`/`SyncDir` extracted from the yaml adapter. The
CLI adds an `mtt agent` group (`hooks` subcommand) and wires the same `Run` into `mtt init` behind a
`--no-agent-hooks` opt-out.

**Tech Stack:** Go 1.23, cobra CLI, `encoding/json` (a `json.Encoder` with `SetEscapeHTML(false)`), `testscript`
e2e, table-driven unit tests.

**Spec:** `docs/superpowers/specs/t52-agent-hooks-scaffold.md` (revision 4, approved).

## Global Constraints

- **Go floor stays 1.23.1** — add no dependency that raises the `go` directive; this uses only the stdlib.
- **Storage stays through ports; this is NOT `.mtt/` storage** — `internal/scaffold` writes agent config files
  outside `.mtt/`, never via `TaskStore`/`KnowledgeStore`, and stays out of `pkg/mtt` (no domain change).
- **One serializer, `SetEscapeHTML(false)`** — the scaffold's settings bytes come from a single
  `json.Encoder` with `SetEscapeHTML(false)` + `SetIndent("", "  ")`; `enc.Encode` already appends the trailing
  `\n` (never double-add). On-disk object keys are alphabetical; `2>/dev/null` is written literally.
- **No clobber** — additive merge only; a malformed or type-incompatible existing settings file is a loud
  refusal, never overwritten.
- **Every `internal/` package keeps its own `CLAUDE.md`** (root `CLAUDE.md:22`) — new packages `internal/fsutil`
  and `internal/scaffold` each get one in this change.
- **`make check` green before every commit** (gofmt + vet + golangci-lint v2 + `go test -race` + build).
- **gosec G304 on new file-IO:** the `.golangci.yml` G304 exclusion is scoped to `_test.go` + `internal/adapter/
  yaml/` only. The extracted `fsutil.SyncDir` (`os.Open`) and `scaffold.Run` (`os.ReadFile`) are outside it, so
  each carries an inline `// #nosec G304 -- <reason>` (the init.go:48 idiom) or the gate goes red.
- **CHANGELOG `[Unreleased]` entry** is required before the implementing→impl_review `mtt submit` (the gate).
- **Commit under the user's git identity**; trailer `Co-Authored-By: Claude Opus 4.8 (1M context)
  <noreply@anthropic.com>`.

---

### Task 1: Extract `internal/fsutil` (shared durable atomic write)

Extract the yaml adapter's atomic-write + dir-fsync discipline into a shared package so the scaffold (Task 3)
reuses it instead of hand-rolling a second durability writer. The yaml adapter's observable behavior stays
byte-identical (its tests + goldens are the safety net); `audit.go`'s direct `syncDir` caller is repointed.

**Files:**
- Create: `internal/fsutil/fsutil.go`
- Create: `internal/fsutil/CLAUDE.md`
- Modify: `internal/adapter/yaml/init.go` (replace `atomicWrite`/`atomicWriteMode`/`syncDir` bodies with thin
  delegations; drop now-unused imports `runtime`, `syscall`)
- Modify: `internal/adapter/yaml/audit.go:60` (`syncDir(dir)` → `fsutil.SyncDir(dir)`; add import)
- Modify: `internal/adapter/yaml/CLAUDE.md` (one-line note: `atomicWrite`/`syncDir` delegate to `internal/fsutil`)
- Test: `internal/fsutil/fsutil_test.go`

**Interfaces:**
- Produces:
  - `func fsutil.AtomicWrite(path string, data []byte, fallbackMode os.FileMode) error` — temp-file + rename +
    fsync; **preserves** an existing non-empty target's mode, else uses `fallbackMode`.
  - `func fsutil.SyncDir(dir string) error` — best-effort parent-dir fsync.

- [ ] **Step 1: Write the failing test** — `internal/fsutil/fsutil_test.go`

```go
package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteCreatesWithFallbackMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := AtomicWrite(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("content = %q, want hello", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %v, want 0644", info.Mode().Perm())
	}
}

func TestAtomicWritePreservesExistingMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWrite(path, []byte("new"), 0o644); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want preserved 0600", info.Mode().Perm())
	}
}

func TestSyncDir(t *testing.T) {
	if err := SyncDir(t.TempDir()); err != nil {
		t.Fatalf("SyncDir: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/fsutil/ -run TestAtomicWrite -v`
Expected: FAIL — build error, `undefined: AtomicWrite` (package doesn't exist yet).

- [ ] **Step 3: Create `internal/fsutil/fsutil.go`** (logic lifted verbatim from `internal/adapter/yaml/init.go`)

```go
// Package fsutil provides a durable, atomic file write shared across adapters:
// temp-file + fsync + rename + parent-dir fsync, with an installer-style mode
// policy (preserve an existing non-empty target's mode, else a fallback). It is
// infrastructure only — no domain types, no storage semantics.
package fsutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

// AtomicWrite writes data to path via a temp file in the same directory + rename,
// with the installer's durability discipline: chmod (preserve an existing
// non-empty target's mode, else fallbackMode), fsync the file before close (the
// rename must never promote un-flushed bytes), and fsync the parent directory
// after the rename so the new directory entry itself survives a crash. A
// zero-byte existing target is treated as absent (its umask-filtered mode must
// not masquerade as a user-tightened one), so fallbackMode wins there.
// MkdirAll of the parent directory is the caller's responsibility.
func AtomicWrite(path string, data []byte, fallbackMode os.FileMode) error {
	mode := fallbackMode
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		mode = info.Mode().Perm()
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".fsutil-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := f.Chmod(mode); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("sync temp: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename temp: %w", err)
	}
	// The rename has landed; a SyncDir failure below surfaces as a write error
	// even though the content is in place. Fail-loud is deliberate: a store that
	// cannot make its writes durable should say so.
	return SyncDir(filepath.Dir(path))
}

// SyncDir fsyncs a directory so a just-renamed entry is durable. Best-effort
// where a directory handle cannot be synced: Windows (FlushFileBuffers on a
// directory fails), filesystems reporting unsupported, and mounts (some
// network/FUSE) that return EINVAL for directory fsync — git tolerates EINVAL
// the same way. The write itself is already flushed; only the entry's
// durability window stays platform-dependent there.
func SyncDir(dir string) error {
	// #nosec G304 -- dir is a caller-supplied local directory path (project root
	// subtree), never network/untrusted input. The yaml adapter's exclusion no
	// longer covers this code once extracted, so suppress inline (init.go idiom).
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open dir %s: %w", dir, err)
	}
	defer func() { _ = d.Close() }()
	if err := d.Sync(); err != nil &&
		!errors.Is(err, errors.ErrUnsupported) && !errors.Is(err, syscall.EINVAL) {
		if runtime.GOOS == "windows" {
			return nil
		}
		return fmt.Errorf("sync dir %s: %w", dir, err)
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/fsutil/ -v`
Expected: PASS (all three tests).

- [ ] **Step 5: Repoint the yaml adapter to `fsutil`** — in `internal/adapter/yaml/init.go`, replace the whole
  `atomicWrite` / `atomicWriteMode` / `syncDir` block (currently lines ~82–167) with thin delegations, keeping
  the `filePerm` const and its doc:

```go
// filePerm is the store's write-perm policy for NEW files (c18): 0644, the
// git-checkout default, so fresh writes and checked-out files agree
// cross-machine. An existing file keeps its own mode (installer-style preserve).
const filePerm = 0o644

// atomicWrite writes data durably at filePerm for a NEW file (an existing
// target's mode is preserved). Delegates to the shared fsutil discipline.
func atomicWrite(path string, data []byte) error {
	return fsutil.AtomicWrite(path, data, filePerm)
}

// atomicWriteMode is atomicWrite with an explicit fallback mode for a NEW file
// (the Current store uses 0600 for a fresh config.local.yaml — personal data up
// to backend credentials). A non-empty existing target's mode still wins.
func atomicWriteMode(path string, data []byte, fallback os.FileMode) error {
	return fsutil.AtomicWrite(path, data, fallback)
}
```

  Then fix imports in `init.go`: add `"github.com/pashukhin/mtt/internal/fsutil"`; remove `"runtime"` and
  `"syscall"` (now unused); keep `"errors"`, `"fmt"`, `"os"`, `"path/filepath"` (still used by `writeGitignore`
  / `writeConfig`). Verify with `goimports`.

- [ ] **Step 6: Repoint `audit.go`'s direct `syncDir` caller** — in `internal/adapter/yaml/audit.go`, change the
  one call at line 60 and add the import:

```go
	// import block: add
	"github.com/pashukhin/mtt/internal/fsutil"
	...
	// line ~60, inside Append:
	if err := fsutil.SyncDir(dir); err != nil {
		return fmt.Errorf("audit: %w", err)
	}
```

- [ ] **Step 7: Create `internal/fsutil/CLAUDE.md`**

```markdown
# internal/fsutil

Shared filesystem durability primitive. Nothing domain-specific lives here.

## Responsibilities

- `AtomicWrite(path, data, fallbackMode)` — temp-file + fsync + rename + parent-dir fsync. Perm policy:
  **preserve** an existing non-empty target's mode, else `fallbackMode` (a zero-byte target counts as absent).
  `MkdirAll` of the parent is the caller's job.
- `SyncDir(dir)` — best-effort parent-dir fsync (tolerates `ErrUnsupported`/`EINVAL`/Windows).

## Boundaries

- No `pkg/mtt` domain types, no `.mtt/` layout knowledge, no business rules — a pure durability helper.
- Extracted from the yaml adapter (was its private `atomicWrite`/`syncDir`); consumers: the yaml adapter
  (task/note/config/audit writes) and `internal/scaffold` (agent settings files).
```

- [ ] **Step 8: Update `internal/adapter/yaml/CLAUDE.md`** — add one sentence to the durability note (near the
  `atomicWrite` mention): "**(t52)** `atomicWrite`/`atomicWriteMode`/`syncDir` now delegate to the shared
  `internal/fsutil` (`AtomicWrite`/`SyncDir`) — same discipline, one implementation; `audit.go` calls
  `fsutil.SyncDir` directly."

- [ ] **Step 9: Run the full gate**

Run: `make check`
Expected: green — the yaml adapter's existing tests/goldens (perm-policy, reserve-artifact, audit fsync) pass
unchanged, proving the extraction is behavior-preserving.

- [ ] **Step 10: Commit**

```bash
git add internal/fsutil internal/adapter/yaml/init.go internal/adapter/yaml/audit.go internal/adapter/yaml/CLAUDE.md
git commit -m "t52: extract internal/fsutil (shared AtomicWrite/SyncDir) from yaml adapter"
```

---

### Task 2: `internal/scaffold` — the `Harness` seam + Claude merge + `Run`

The pure `Merge` (idempotent, no-clobber, one serializer) and the thin IO `Run`, plus the `Harness` interface +
registry (the extension seam; only `claude` registered — no guessed codex/gemini config).

**Files:**
- Create: `internal/scaffold/scaffold.go` (`Action`, `Result`, `Harness`, `Registry`, `Run`)
- Create: `internal/scaffold/claude.go` (the Claude `Harness` + `Merge`)
- Create: `internal/scaffold/CLAUDE.md`
- Test: `internal/scaffold/claude_test.go`, `internal/scaffold/run_test.go`

**Interfaces:**
- Consumes: `fsutil.AtomicWrite` (Task 1).
- Produces:
  - `type Action string` with `Created`/`Merged`/`Unchanged` (values `"created"/"merged"/"unchanged"`).
  - `type Result struct { Harness, Path string; Action Action }`.
  - `type Harness interface { Name() string; RelPath() string; Merge(existing []byte, exists bool) ([]byte, Action, error) }`.
  - `func Registry() []Harness`.
  - `func Run(root string, harnesses []Harness) ([]Result, error)`.

- [ ] **Step 1: Write the failing merge tests** — `internal/scaffold/claude_test.go` (table-driven, byte-exact)

```go
package scaffold

import (
	"strings"
	"testing"
)

// createdBytes is the canonical Created output: alphabetical object keys,
// 2-space indent, trailing newline, literal 2>/dev/null (no HTML escaping).
const createdBytes = `{
  "hooks": {
    "PreCompact": [
      {
        "hooks": [
          {
            "command": "mtt prime 2>/dev/null || true",
            "type": "command"
          }
        ]
      }
    ],
    "SessionStart": [
      {
        "hooks": [
          {
            "command": "mtt prime 2>/dev/null || true",
            "type": "command"
          }
        ]
      }
    ]
  },
  "permissions": {
    "allow": [
      "Bash(mtt roadmap:*)",
      "Bash(mtt list:*)",
      "Bash(mtt show:*)",
      "Bash(mtt tree:*)",
      "Bash(mtt ready:*)",
      "Bash(mtt types:*)",
      "Bash(mtt prime:*)",
      "Bash(mtt check:*)",
      "Bash(mtt tags:*)",
      "Bash(mtt version:*)",
      "Bash(mtt dep list:*)",
      "Bash(mtt note show:*)",
      "Bash(mtt note list:*)",
      "Bash(mtt ref list:*)",
      "Bash(mtt note ref list:*)"
    ]
  }
}
`

func TestMergeCreatedFromAbsent(t *testing.T) {
	got, action, err := claudeHarness{}.Merge(nil, false)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if action != Created {
		t.Fatalf("action = %q, want created", action)
	}
	if string(got) != createdBytes {
		t.Fatalf("bytes mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, createdBytes)
	}
	if !strings.Contains(string(got), "2>/dev/null") {
		t.Fatal("literal 2>/dev/null must survive (no HTML escaping)")
	}
}

func TestMergeConvergence(t *testing.T) {
	// R1: the Created output is the serializer's fixed point.
	_, action, err := claudeHarness{}.Merge([]byte(createdBytes), true)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if action != Unchanged {
		t.Fatalf("action = %q, want unchanged", action)
	}
}

func TestMergeReorderConvergesInOneStep(t *testing.T) {
	// spec test 5: a hand-authored file carrying our hooks but with type-before-
	// command key order (and no permissions) ⇒ exactly one Merged (re-sort + add
	// the allowlist), then Unchanged.
	h := claudeHarness{}
	in := `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"mtt prime 2>/dev/null || true"}]}],` +
		`"PreCompact":[{"hooks":[{"type":"command","command":"mtt prime 2>/dev/null || true"}]}]}}`
	first, a1, err := h.Merge([]byte(in), true)
	if err != nil {
		t.Fatalf("Merge(1): %v", err)
	}
	if a1 != Merged {
		t.Fatalf("first = %q, want merged", a1)
	}
	_, a2, err := h.Merge(first, true)
	if err != nil {
		t.Fatalf("Merge(2): %v", err)
	}
	if a2 != Unchanged {
		t.Fatalf("second = %q, want unchanged", a2)
	}
}

func TestMergeEmptyFileTreatedAsAbsent(t *testing.T) {
	got, action, err := claudeHarness{}.Merge([]byte("   \n\t"), true)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if action != Created || string(got) != createdBytes {
		t.Fatalf("empty file must create; got action %q", action)
	}
}

func TestMergePreservesUnrelatedKeys(t *testing.T) {
	in := `{"env":{"FOO":"bar"}}`
	got, action, err := claudeHarness{}.Merge([]byte(in), true)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if action != Merged {
		t.Fatalf("action = %q, want merged", action)
	}
	if !strings.Contains(string(got), `"env"`) || !strings.Contains(string(got), `"FOO"`) {
		t.Fatalf("unrelated keys dropped:\n%s", got)
	}
	if !strings.Contains(string(got), "SessionStart") {
		t.Fatalf("our hook not added:\n%s", got)
	}
}

func TestMergeCustomPrimeArgsNoDuplicate(t *testing.T) {
	in := `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"mtt prime --limit 5"}]}],` +
		`"PreCompact":[{"hooks":[{"type":"command","command":"mtt prime --limit 5"}]}]}}`
	got, _, err := claudeHarness{}.Merge([]byte(in), true)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	// Count the exact custom command (NOT the bare "mtt prime" substring, which
	// also appears in the "Bash(mtt prime:*)" allowlist entry): the two existing
	// hooks stay, and our guarded command is NOT appended (they satisfy presence).
	if n := strings.Count(string(got), "mtt prime --limit 5"); n != 2 {
		t.Fatalf("expected the 2 existing custom prime hooks, no duplicate; got %d:\n%s", n, got)
	}
	if strings.Contains(string(got), "mtt prime 2>/dev/null") {
		t.Fatalf("our guarded hook should not be appended when a prime hook is already present:\n%s", got)
	}
}

func TestMergeWordBoundaryNotPresent(t *testing.T) {
	// r5/d4: a mention not-at-start, and a longer word, must NOT count as present.
	for _, cmd := range []string{`git commit -m "wire mtt prime"`, "mtt primer"} {
		in := `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":` +
			mustJSONString(cmd) + `}]}]}}`
		got, action, err := claudeHarness{}.Merge([]byte(in), true)
		if err != nil {
			t.Fatalf("Merge(%q): %v", cmd, err)
		}
		if action != Merged || strings.Count(string(got), "SessionStart") != 1 {
			t.Fatalf("cmd %q: our hook should be appended; action %q:\n%s", cmd, action, got)
		}
		// The event now has the mention + our hook => two commands under SessionStart.
		if !strings.Contains(string(got), "mtt prime 2>/dev/null") {
			t.Fatalf("cmd %q: our prime command missing:\n%s", cmd, got)
		}
	}
}

func TestMergeMalformedJSON(t *testing.T) {
	// NB: a composite literal (claudeHarness{}) may not appear in an if-init
	// header (Go parse ambiguity) — hoist it to a local.
	h := claudeHarness{}
	if _, _, err := h.Merge([]byte("{not json"), true); err == nil {
		t.Fatal("malformed JSON must error")
	}
}

func TestMergeRefusesIncompatibleShape(t *testing.T) {
	h := claudeHarness{}
	for _, in := range []string{
		`{"hooks":"oops"}`,
		`{"hooks":{"SessionStart":"oops"}}`,
		`{"permissions":{"allow":"oops"}}`,
	} {
		if _, _, err := h.Merge([]byte(in), true); err == nil {
			t.Fatalf("incompatible shape %q must refuse", in)
		}
	}
}

func TestMergeNullContainerTreatedAsAbsent(t *testing.T) {
	h := claudeHarness{}
	for _, in := range []string{
		`{"hooks":null}`,
		`{"hooks":{"SessionStart":null}}`,
		`{"permissions":null}`,
		`{"permissions":{"allow":null}}`,
	} {
		if _, _, err := h.Merge([]byte(in), true); err != nil {
			t.Fatalf("null container %q must be treated as absent, got err %v", in, err)
		}
	}
}

func TestMergeToleratesOddShapes(t *testing.T) {
	// valid JSON, odd shapes the presence walk must survive without panic.
	h := claudeHarness{}
	for _, in := range []string{
		`{"hooks":{"SessionStart":[{}]}}`,
		`{"hooks":{"SessionStart":[{"hooks":[{"command":42}]}]}}`,
	} {
		if _, _, err := h.Merge([]byte(in), true); err != nil {
			t.Fatalf("odd shape %q should not error, got %v", in, err)
		}
	}
}

func TestMergePermissionsUnionDedup(t *testing.T) {
	in := `{"permissions":{"allow":["Bash(mtt roadmap:*)","Custom(x)"]}}`
	got, _, err := claudeHarness{}.Merge([]byte(in), true)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if strings.Count(string(got), "Bash(mtt roadmap:*)") != 1 {
		t.Fatalf("roadmap entry should be deduped:\n%s", got)
	}
	if !strings.Contains(string(got), "Custom(x)") {
		t.Fatalf("existing custom entry dropped:\n%s", got)
	}
	// existing entries come first.
	if strings.Index(string(got), "Custom(x)") > strings.Index(string(got), "Bash(mtt list:*)") {
		t.Fatalf("existing entry should precede our new ones:\n%s", got)
	}
}

func TestMergeOneEventPresent(t *testing.T) {
	in := `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"mtt prime"}]}]}}`
	got, action, err := claudeHarness{}.Merge([]byte(in), true)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if action != Merged {
		t.Fatalf("action = %q, want merged", action)
	}
	if strings.Count(string(got), "PreCompact") != 1 {
		t.Fatalf("PreCompact should be added exactly once:\n%s", got)
	}
}
```

- [ ] **Step 2: Add the tiny test helper** — append to `claude_test.go` (and add `"encoding/json"` to that
  file's imports, alongside `"strings"` and `"testing"`):

```go
// mustJSONString returns s as a JSON string literal (with quotes), for building
// table fixtures whose command contains quotes/backslashes safely.
func mustJSONString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/scaffold/ -v`
Expected: FAIL — `undefined: claudeHarness` / `Created` / package empty.

- [ ] **Step 4: Create `internal/scaffold/claude.go`**

```go
package scaffold

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// primeCommand is the guarded session-start command (a missing mtt binary never
// nags a contributor).
const primeCommand = "mtt prime 2>/dev/null || true"

// hookEvents are the Claude Code events we hang the prime hook on: session start
// and around compaction (SessionStart also re-fires with source=compact).
var hookEvents = []string{"SessionStart", "PreCompact"}

// readOnlyAllow is the read-only mtt permission allowlist (granular per
// subcommand; every mutating verb is deliberately excluded).
var readOnlyAllow = []string{
	"Bash(mtt roadmap:*)", "Bash(mtt list:*)", "Bash(mtt show:*)", "Bash(mtt tree:*)",
	"Bash(mtt ready:*)", "Bash(mtt types:*)", "Bash(mtt prime:*)", "Bash(mtt check:*)",
	"Bash(mtt tags:*)", "Bash(mtt version:*)", "Bash(mtt dep list:*)",
	"Bash(mtt note show:*)", "Bash(mtt note list:*)", "Bash(mtt ref list:*)", "Bash(mtt note ref list:*)",
}

// claudeHarness scaffolds Claude Code's .claude/settings.json.
type claudeHarness struct{}

func (claudeHarness) Name() string    { return "claude" }
func (claudeHarness) RelPath() string { return ".claude/settings.json" }

// Merge computes the desired settings bytes from the current ones. Pure: no IO.
// Create-if-absent (or empty), additive union if present, Unchanged if already
// there, loud refusal if malformed or type-incompatible — never a clobber.
func (claudeHarness) Merge(existing []byte, exists bool) ([]byte, Action, error) {
	created := !exists || len(bytes.TrimSpace(existing)) == 0
	m := map[string]any{}
	if !created {
		if err := json.Unmarshal(existing, &m); err != nil {
			return nil, "", fmt.Errorf("parse existing settings: %w", err)
		}
	}
	if err := ensureHooks(m); err != nil {
		return nil, "", err
	}
	if err := ensurePermissions(m); err != nil {
		return nil, "", err
	}
	out, err := encode(m)
	if err != nil {
		return nil, "", err
	}
	switch {
	case created:
		return out, Created, nil
	case bytes.Equal(out, existing):
		return existing, Unchanged, nil
	default:
		return out, Merged, nil
	}
}

// encode is THE serializer for both create and merge (R1): SetEscapeHTML(false)
// keeps 2>/dev/null literal, SetIndent gives 2-space indent, Encode appends \n.
// Object keys sort alphabetically (encoding/json) — the fixed point that makes
// the Unchanged byte-equality converge.
func encode(m map[string]any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return nil, fmt.Errorf("encode settings: %w", err)
	}
	return buf.Bytes(), nil
}

// ensureHooks adds our prime hook to each event, preserving existing entries.
// A null hooks value is treated as absent; a non-null non-object refuses.
func ensureHooks(m map[string]any) error {
	hooks, err := childObject(m, "hooks")
	if err != nil {
		return err
	}
	m["hooks"] = hooks
	for _, event := range hookEvents {
		if err := ensureEventHook(hooks, event); err != nil {
			return err
		}
	}
	return nil
}

// ensureEventHook appends our matcher-group to an event array unless a prime
// hook is already present. A null event value is treated as absent; a non-null
// non-array refuses.
func ensureEventHook(hooks map[string]any, event string) error {
	arr, err := childArray(hooks, event)
	if err != nil {
		return err
	}
	if eventHasPrime(arr) {
		hooks[event] = arr
		return nil
	}
	hooks[event] = append(arr, map[string]any{
		"hooks": []any{
			map[string]any{"type": "command", "command": primeCommand},
		},
	})
	return nil
}

// ensurePermissions unions our read-only allowlist into permissions.allow,
// deduping by exact string, existing entries first.
func ensurePermissions(m map[string]any) error {
	perms, err := childObject(m, "permissions")
	if err != nil {
		return err
	}
	m["permissions"] = perms
	allow, err := childArray(perms, "allow")
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, e := range allow {
		if s, ok := e.(string); ok {
			seen[s] = true
		}
	}
	for _, entry := range readOnlyAllow {
		if !seen[entry] {
			allow = append(allow, entry)
			seen[entry] = true
		}
	}
	perms["allow"] = allow
	return nil
}

// childObject returns m[key] as an object, creating it when absent or null, and
// refusing a non-null non-object (a shape we cannot safely extend).
func childObject(m map[string]any, key string) (map[string]any, error) {
	raw, ok := m[key]
	if !ok || raw == nil {
		return map[string]any{}, nil
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("cannot merge: %q is not a JSON object", key)
	}
	return obj, nil
}

// childArray returns m[key] as an array, creating it when absent or null, and
// refusing a non-null non-array.
func childArray(m map[string]any, key string) ([]any, error) {
	raw, ok := m[key]
	if !ok || raw == nil {
		return []any{}, nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("cannot merge: %q is not a JSON array", key)
	}
	return arr, nil
}

// eventHasPrime reports whether any command under the event is our prime hook
// (word-boundary prefix, r5). Tolerates arbitrary shapes (never panics).
func eventHasPrime(arr []any) bool {
	for _, entry := range arr {
		group, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		inner, ok := group["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range inner {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			cmd, ok := hm["command"].(string)
			if ok && isPrimeCommand(cmd) {
				return true
			}
		}
	}
	return false
}

// isPrimeCommand is a word-boundary prefix match (r5/d4): the trimmed command
// equals "mtt prime" or begins with "mtt prime " (a trailing space), so
// "mtt prime --limit 5" matches but "mtt primer" and an incidental mention do not.
func isPrimeCommand(cmd string) bool {
	c := strings.TrimLeft(cmd, " \t")
	return c == "mtt prime" || strings.HasPrefix(c, "mtt prime ")
}
```

- [ ] **Step 5: Create `internal/scaffold/scaffold.go`**

```go
// Package scaffold writes agent-facing configuration (hooks + permissions) for
// supported harnesses. It is onboarding infrastructure: it writes files outside
// .mtt/ (via internal/fsutil), never through a TaskStore/KnowledgeStore, and
// carries no pkg/mtt domain. The Harness interface + Registry are the extension
// seam; only "claude" is registered (codex/gemini deferred until verified).
package scaffold

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pashukhin/mtt/internal/fsutil"
)

// Action is what Merge/Run did to a harness's file.
type Action string

// The three merge outcomes (also the human/JSON labels).
const (
	Created   Action = "created"
	Merged    Action = "merged"
	Unchanged Action = "unchanged"
)

// Result is the per-harness scaffold outcome.
type Result struct {
	Harness string
	Path    string
	Action  Action
}

// Harness is a scaffold target (one agent tool's config). Merge is pure.
type Harness interface {
	Name() string
	RelPath() string
	Merge(existing []byte, exists bool) (content []byte, action Action, err error)
}

// Registry is the set of supported harnesses (the single extension point).
func Registry() []Harness { return []Harness{claudeHarness{}} }

// Run scaffolds each harness under root and returns per-harness results. It is
// write-as-you-go: a read/merge/write failure aborts the loop and returns the
// results collected so far alongside a wrapped error. Only os.ErrNotExist marks
// a file absent; any other read error propagates.
func Run(root string, harnesses []Harness) ([]Result, error) {
	var results []Result
	for _, h := range harnesses {
		path := filepath.Join(root, filepath.FromSlash(h.RelPath()))
		// #nosec G304 -- path is <root>/<harness RelPath> under a locally-resolved
		// project root (projectRoot/baseDir), not network/untrusted input.
		existing, err := os.ReadFile(path)
		exists := true
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				exists, existing = false, nil
			} else {
				return results, fmt.Errorf("%s: read %s: %w", h.Name(), path, err)
			}
		}
		content, action, err := h.Merge(existing, exists)
		if err != nil {
			return results, fmt.Errorf("%s: %s: %w", h.Name(), path, err)
		}
		if action != Unchanged {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return results, fmt.Errorf("%s: mkdir %s: %w", h.Name(), filepath.Dir(path), err)
			}
			if err := fsutil.AtomicWrite(path, content, 0o644); err != nil {
				return results, fmt.Errorf("%s: write %s: %w", h.Name(), path, err)
			}
		}
		results = append(results, Result{Harness: h.Name(), Path: path, Action: action})
	}
	return results, nil
}
```

- [ ] **Step 6: Write the `Run` tests** — `internal/scaffold/run_test.go`

```go
package scaffold

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunCreatesThenIdempotent(t *testing.T) {
	root := t.TempDir()
	res, err := Run(root, Registry())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res) != 1 || res[0].Action != Created {
		t.Fatalf("first run = %+v, want one Created", res)
	}
	path := filepath.Join(root, ".claude", "settings.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("settings not written: %v", err)
	}
	res2, err := Run(root, Registry())
	if err != nil {
		t.Fatalf("Run(2): %v", err)
	}
	if res2[0].Action != Unchanged {
		t.Fatalf("second run = %q, want Unchanged", res2[0].Action)
	}
}

func TestRunPropagatesNonNotExistReadError(t *testing.T) {
	// r6/d3: a .claude/settings.json that is a DIRECTORY is a non-IsNotExist read
	// error and must propagate, not be mistaken for absent.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude", "settings.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(root, Registry()); err == nil {
		t.Fatal("expected a read error to propagate")
	}
}
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test ./internal/scaffold/ -race -v`
Expected: PASS (all merge + run cases).

- [ ] **Step 8: Create `internal/scaffold/CLAUDE.md`**

```markdown
# internal/scaffold

Scaffolds agent-facing configuration (hooks + permissions) for supported harnesses. Onboarding
infrastructure — NOT tasks/knowledge domain.

## Responsibilities

- `Harness` (seam) + `Registry()` — the extension point. Only `claude` is registered; codex/gemini are
  deferred until their config-hook formats are verified (no guessed config is written).
- `claudeHarness.Merge` (pure) — additive, idempotent, no-clobber merge of `.claude/settings.json`:
  `SessionStart`+`PreCompact` → `mtt prime 2>/dev/null || true`, plus a read-only `permissions.allow`. One
  `json.Encoder` (`SetEscapeHTML(false)`, 2-space indent) — alphabetical keys, literal `2>/dev/null`.
- `Run(root, harnesses)` — the thin IO edge (read → Merge → `fsutil.AtomicWrite`), write-as-you-go.

## Boundaries

- No `pkg/mtt` domain, no `.mtt/` storage, no port. Writes via `internal/fsutil`.
- Presence is a word-boundary `mtt prime` prefix; malformed / type-incompatible existing files are refused,
  never clobbered; a null/empty container is treated as absent.
```

- [ ] **Step 9: Run the gate & commit**

```bash
make check
git add internal/scaffold
git commit -m "t52: internal/scaffold — Harness seam + Claude settings merge + Run"
```

---

### Task 3: `mtt agent hooks` command

Wire the `mtt agent` group + `hooks` subcommand over `scaffold.Run`, with `--json` and the shared reporter.

**Files:**
- Create: `internal/cli/agent.go` (`newAgentCmd`, `newAgentHooksCmd`, `reportScaffold`, `scaffoldJSON`, `toScaffoldJSON`)
- Modify: `internal/cli/root.go:46-49` (register `newAgentCmd()`)
- Modify: `internal/cli/CLAUDE.md` (document the command)
- Test: `internal/cli/testdata/scripts/agent_hooks.txt`

**Interfaces:**
- Consumes: `scaffold.Run`, `scaffold.Registry` (Task 2); `projectRoot`, `jsonFlag`, `writeJSON` (existing).
- Produces: `func toScaffoldJSON([]scaffold.Result) []scaffoldJSON` and `func reportScaffold(*cobra.Command,
  []scaffold.Result) error` (reused by Task 4's init).

- [ ] **Step 1: Write the failing e2e** — `internal/cli/testdata/scripts/agent_hooks.txt`

```
# agent hooks scaffolds Claude settings into an initialized project
mkdir proj
cd proj
exec mtt init --no-agent-hooks
! exists .claude/settings.json
exec mtt agent hooks
stdout 'claude: created'
exists .claude/settings.json
grep '"SessionStart"' .claude/settings.json
grep '"PreCompact"' .claude/settings.json
grep 'mtt prime 2>/dev/null' .claude/settings.json
grep 'Bash\(mtt roadmap:\*\)' .claude/settings.json
# the mutating verbs must NOT be granted
! grep 'mtt note add' .claude/settings.json

# a second run is idempotent
exec mtt agent hooks
stdout 'claude: unchanged'

# --json surfaces the result array
exec mtt agent hooks --json
stdout '"harness": "claude"'
stdout '"action": "unchanged"'

# outside a project: actionable error (m6)
cd $WORK
mkdir empty
cd empty
! exec mtt agent hooks
stderr 'mtt init'
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/cli/ -run 'TestScripts/agent_hooks' -v`
Expected: FAIL — `unknown command "agent"`.

- [ ] **Step 3: Create `internal/cli/agent.go`**

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/scaffold"
)

// newAgentCmd builds `mtt agent` — the onboarding group for agent-facing
// artifacts (t52: `hooks`; t46 later adds `docs`).
func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Scaffold agent-facing configuration for coding-agent harnesses",
	}
	cmd.AddCommand(newAgentHooksCmd())
	return cmd
}

// newAgentHooksCmd builds `mtt agent hooks` — scaffold settings + hooks
// (SessionStart/PreCompact -> mtt prime) + a read-only permissions allowlist for
// every registered harness (currently Claude Code). Idempotent; re-runnable.
func newAgentHooksCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "hooks",
		Short: "Scaffold agent settings + hooks (SessionStart/PreCompact -> mtt prime) for registered harnesses",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			results, err := scaffold.Run(root, scaffold.Registry())
			if err != nil {
				return err
			}
			return reportScaffold(cmd, results)
		},
	}
}

// scaffoldJSON is the machine-readable per-harness scaffold result.
type scaffoldJSON struct {
	Harness string `json:"harness"`
	Path    string `json:"path"`
	Action  string `json:"action"`
}

// toScaffoldJSON maps results to the non-null JSON view (empty slice, not null).
func toScaffoldJSON(results []scaffold.Result) []scaffoldJSON {
	out := make([]scaffoldJSON, 0, len(results))
	for _, r := range results {
		out = append(out, scaffoldJSON{Harness: r.Harness, Path: r.Path, Action: string(r.Action)})
	}
	return out
}

// reportScaffold renders the results (shared by `agent hooks` and `init`).
// Human: one line per harness + a single note when anything changed. JSON: the
// non-null result array.
func reportScaffold(cmd *cobra.Command, results []scaffold.Result) error {
	if jsonFlag(cmd) {
		return writeJSON(cmd.OutOrStdout(), toScaffoldJSON(results))
	}
	changed := false
	for _, r := range results {
		if r.Action != scaffold.Unchanged {
			changed = true
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s: %s %s\n", r.Harness, r.Action, r.Path); err != nil {
			return err
		}
	}
	if changed {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(),
			"→ a SessionStart/PreCompact hook now runs `mtt prime` at session start; review the settings file.")
	}
	return nil
}
```

- [ ] **Step 4: Register the command** — in `internal/cli/root.go`, add `newAgentCmd()` to the `AddCommand`
  call (e.g. after `newSelfUpdateCmd()`):

```go
	root.AddCommand(newVersionCmd(), newInitCmd(), newTypesCmd(), newAddCmd(), newShowCmd(),
		newListCmd(), newEditCmd(), newTreeCmd(), newDepCmd(), newReadyCmd(), newStatusCmd(),
		newUseCmd(), newRmCmd(), newRoadmapCmd(), newTagCmd(), newTagsCmd(), newDoCmd(),
		newNoteCmd(), newRefCmd(), newCheckCmd(), newPrimeCmd(), newSelfUpdateCmd(), newAgentCmd())
```

- [ ] **Step 5: Run the e2e to verify it passes**

Run: `go test ./internal/cli/ -run 'TestScripts/agent_hooks' -v`
Expected: PASS.

- [ ] **Step 6: Document in `internal/cli/CLAUDE.md`** — add a short paragraph:

```markdown
Agent scaffolding (t52): `newAgentCmd` (`agent.go`) is the `mtt agent` group (the `note`/`dep` group pattern);
`mtt agent hooks` scaffolds `.claude/settings.json` for every `scaffold.Registry()` harness via `scaffold.Run`
(root through `projectRoot`), rendering per-harness `created|merged|unchanged` (shared `reportScaffold`/
`scaffoldJSON`/`toScaffoldJSON`, reused by `init`). No business logic here — merge/IO live in `internal/scaffold`.
```

- [ ] **Step 7: Gate & commit**

```bash
make check
git add internal/cli/agent.go internal/cli/root.go internal/cli/CLAUDE.md internal/cli/testdata/scripts/agent_hooks.txt
git commit -m "t52: mtt agent hooks command"
```

---

### Task 4: `mtt init` scaffolds by default (+ `--no-agent-hooks`, failure contract)

Make `init` run the same scaffold after the config write, opt-out `--no-agent-hooks`; fold results into output
and pin the config-then-scaffold failure contract.

**Files:**
- Modify: `internal/cli/init.go` (add flag; call `scaffold.Run` after the switch; failure contract; output)
- Modify: `internal/cli/json.go:114-120` (`initJSON` gains `Scaffold []scaffoldJSON`)
- Modify: `internal/cli/testdata/scripts/init.txt` (scaffold-by-default, opt-out, malformed-failure)
- Test: extend `internal/cli/init.txt` (above) — no new file.

**Interfaces:**
- Consumes: `scaffold.Run`, `scaffold.Registry`, `toScaffoldJSON`, `reportScaffold` (Task 3).

- [ ] **Step 1: Extend the init e2e** — in `internal/cli/testdata/scripts/init.txt`, **insert this command
  block at the END of the command section, immediately BEFORE the first `-- ` txtar marker** (the existing
  `-- empty/.keep --`). Commands placed after a `--` marker parse as file data and would silently never run
  (false green), so ordering matters:

```
# --- t52: init scaffolds agent hooks by default ---
cd $WORK
mkdir scaffoldproj
cd scaffoldproj
exec mtt init
stdout 'initialized'
stdout 'claude: created'
exists .claude/settings.json
grep 'mtt prime 2>/dev/null' .claude/settings.json

# --no-agent-hooks opts out
cd $WORK
mkdir noscaffold
cd noscaffold
exec mtt init --no-agent-hooks
stdout 'initialized'
! exists .claude/settings.json

# init --json carries a non-null scaffold array
cd $WORK
mkdir jsonscaffold
cd jsonscaffold
exec mtt init --json
stdout '"scaffold"'
stdout '"harness": "claude"'

# R3: a pre-existing malformed .claude/settings.json fails the scaffold, but
# config is written (primary job) and reported; exit non-zero; no partial JSON.
cd $WORK
mkdir malformed
cd malformed
mkdir .claude
cp $WORK/badsettings.json .claude/settings.json
! exec mtt init
stdout 'initialized'
stderr 'parse existing settings'
exists .mtt/config.yaml
```

  and add the fixture file **at the very bottom of `init.txt`, after the existing `-- bad.yaml --` fixture**
  (all txtar `-- file --` blocks live together at the end):

```
-- badsettings.json --
{not valid json
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/cli/ -run 'TestScripts/init' -v`
Expected: FAIL — `unknown flag: --no-agent-hooks` / no `claude: created` line.

- [ ] **Step 3: Add the flag and wire the scaffold** — in `internal/cli/init.go`, add the flag var and
  registration:

```go
	var (
		tmpl         string
		force        bool
		name         string
		autoYes      bool
		noAgentHooks bool
	)
	...
	cmd.Flags().BoolVar(&noAgentHooks, "no-agent-hooks", false, "skip scaffolding agent settings + hooks (.claude/settings.json)")
```

  Then replace the current output tail (the `if jsonFlag(cmd) { ... }` block through the final human print) with
  the config-then-scaffold sequence:

```go
			// The config write (the switch above) is init's primary job and has
			// succeeded here. Scaffold agent hooks by default (opt-out flag), then
			// fold the results into output. A scaffold failure (a pre-existing
			// malformed .claude/settings.json) is reported AFTER the config success
			// and returns exit 1 — config is kept, no rollback (R3).
			var scaffoldResults []scaffold.Result
			if !noAgentHooks {
				var serr error
				scaffoldResults, serr = scaffold.Run(base, scaffold.Registry())
				if serr != nil {
					if !jsonFlag(cmd) {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "initialized .mtt/config.yaml (template %q)\n", tmpl)
					}
					return serr
				}
			}
			if jsonFlag(cmd) {
				absBase, err := filepath.Abs(base)
				if err != nil {
					return err
				}
				return writeJSON(cmd.OutOrStdout(), initJSON{
					Path:     filepath.Join(absBase, ".mtt", "config.yaml"),
					Template: tmpl, Source: source, Name: projectName, Created: true,
					Scaffold: toScaffoldJSON(scaffoldResults),
				})
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "initialized .mtt/config.yaml (template %q)\n", tmpl); err != nil {
				return err
			}
			return reportScaffold(cmd, scaffoldResults)
```

  Add `"github.com/pashukhin/mtt/internal/scaffold"` to `init.go`'s imports.

  Note on `reportScaffold` for the human tail: it prints per-harness lines to stdout + the change-note to
  stderr. With `--no-agent-hooks`, `scaffoldResults` is nil ⇒ it prints nothing (correct). The `initialized …`
  line is printed just above, so ordering is config-line then scaffold-lines.

- [ ] **Step 4: Extend `initJSON`** — in `internal/cli/json.go`:

```go
// initJSON is `mtt init --json`: the created-config summary + agent-scaffold
// results (non-null array; [] when --no-agent-hooks).
type initJSON struct {
	Path     string         `json:"path"`
	Template string         `json:"template"`
	Source   string         `json:"source,omitempty"`
	Name     string         `json:"name"`
	Created  bool           `json:"created"`
	Scaffold []scaffoldJSON `json:"scaffold"`
}
```

  `toScaffoldJSON(nil)` returns a non-null empty slice, so `--no-agent-hooks --json` emits `"scaffold": []`.

- [ ] **Step 5: Run the e2e to verify it passes**

Run: `go test ./internal/cli/ -run 'TestScripts/init' -v`
Expected: PASS.

- [ ] **Step 6: Guard the existing init unit test** — the `init_test.go` `TestInitCommand` calls `runRoot(t,
  "init")` in a temp cwd; it will now also create `.claude/settings.json` there (harmless). No change needed,
  but confirm it still passes:

Run: `go test ./internal/cli/ -run TestInitCommand -v`
Expected: PASS.

- [ ] **Step 7: Gate & commit**

```bash
make check
git add internal/cli/init.go internal/cli/json.go internal/cli/testdata/scripts/init.txt
git commit -m "t52: mtt init scaffolds agent hooks by default (--no-agent-hooks)"
```

---

### Task 5: Docs sync (behavior change)

Reconcile the docs with the shipped behavior — including the parallel restated "hook is config, not code"
claims (the project's logged docs-sync lesson) — and the CHANGELOG gate.

**Files:**
- Modify: `DESIGN.md` (+ `DESIGN.ru.md`) — new Shipped(t52) note; amend the stale t51 claim at `DESIGN.md:957-958`
  / `DESIGN.ru.md:~973` (grep `config, not code` / `конфиг, не` to confirm — line numbers drift). Leave
  `DESIGN.md:508` / `.ru:514` (t26 auto-commit, a different fact).
- Modify: `FLOW_GUIDE.md:292` (+ `FLOW_GUIDE.ru.md:295`) — flip the "Settings & hooks" future bullet to Shipped/t52.
- Modify: `CLI_REFERENCE.md` (+ `.ru`) — document `mtt agent hooks` + `mtt init --no-agent-hooks`; update the t51
  "wire it into session start" snippet to point at the command.
- Modify: `internal/cli/CLAUDE.md:294` — amend the stale "config … not code" line.
- Modify: `CHANGELOG.md` — `[Unreleased]` entry.
- Modify: `README.md` (+ `.ru`) — if the Quickstart enumerates init behavior, note the default scaffold + opt-out.

(No code, no TDD steps — prose. `make check` still run to keep the build/CHANGELOG-gate honest.)

- [ ] **Step 1: DESIGN.md — add the Shipped(t52) note** (after the t62 Shipped block, near the init/onboarding
  section). Cover: the `mtt agent` group + `hooks`; `mtt init` default + `--no-agent-hooks`; the Claude flagship;
  `.claude/settings.json` = SessionStart+PreCompact→`mtt prime` + read-only `permissions.allow`; the additive,
  idempotent, no-clobber merge (one `json.Encoder`/`SetEscapeHTML(false)`/alphabetical keys; word-boundary
  presence); `internal/scaffold` `Harness` seam + registry (codex/gemini deferred, no guessed config);
  `internal/fsutil` extraction (DRY, `audit.go` repointed); the release-bar reporting + init failure contract.

- [ ] **Step 2: DESIGN.md — amend the stale t51 claim** at lines 957-958: change "The `sessionStart` hook is
  config, not code — mtt ships only the command" to note t52 makes it **also** scaffolded (config **and** code —
  `mtt agent hooks` / init default emit it; still user-overridable). Do NOT touch line 508 (t26 fact).

- [ ] **Step 3: Mirror Steps 1-2 in `DESIGN.ru.md`** (Shipped(t52) note + amend the stale claim near line ~973
  — grep `конфиг, не` / `sessionStart` to locate). Keep EN/RU in sync.

- [ ] **Step 4: FLOW_GUIDE.md:292 + FLOW_GUIDE.ru.md:295** — flip the "Settings & hooks — scaffolding
  editor/agent settings and hooks (`sessionStart → mtt prime`)" bullet from the future/"still tracked
  separately" list to a Shipped/t52 statement (or move it out of the future list).

- [ ] **Step 5: CLI_REFERENCE.md (+ .ru)** — add an `mtt agent hooks` entry (what it writes, idempotent,
  no-clobber, `projectRoot`) and the `mtt init --no-agent-hooks` flag; update the t51 "Wire it into session
  start" section (the `settings.json` snippet) to say the hook is now scaffolded by `mtt agent hooks` (init
  default), the manual snippet being the fallback.

- [ ] **Step 6: internal/cli/CLAUDE.md:294** — amend "`sessionStart` hook is config (documented in
  CLI_REFERENCE), not code" to note it is now also scaffolded by `mtt agent hooks`/init (t52).

- [ ] **Step 7: CHANGELOG.md** — under `[Unreleased]`:

```markdown
### Added
- `mtt agent hooks` scaffolds Claude Code `.claude/settings.json` — `SessionStart`+`PreCompact` hooks running
  `mtt prime`, plus a read-only `permissions.allow` — via an additive, idempotent, no-clobber merge. `mtt init`
  runs it by default (`--no-agent-hooks` to opt out). Extensible `Harness` seam (codex/gemini deferred).

### Changed
- Extracted `internal/fsutil` (`AtomicWrite`/`SyncDir`) shared by the yaml adapter and the scaffold.
```

- [ ] **Step 8: README.md (+ .ru)** — if the Quickstart lists what `mtt init` does, add one line: it also
  scaffolds `.claude/settings.json` (hooks + read-only allowlist) unless `--no-agent-hooks`.

- [ ] **Step 9: Gate & commit**

```bash
make check
git add DESIGN.md DESIGN.ru.md FLOW_GUIDE.md FLOW_GUIDE.ru.md CLI_REFERENCE.md CLI_REFERENCE.ru.md internal/cli/CLAUDE.md CHANGELOG.md README.md README.ru.md
git commit -m "t52: docs — mtt agent hooks + init default; sync DESIGN/FLOW_GUIDE/CLI_REFERENCE/README (EN+RU)"
```

---

## Self-Review

**Spec coverage** (each spec section → a task):
- Surface (`mtt agent hooks`, init default + `--no-agent-hooks`, root resolution, target file) → Tasks 3, 4.
- What is written (hooks + allowlist; literal `2>/dev/null`; alphabetical bytes) → Task 2 (`claude.go`, tests).
- Architecture (`internal/scaffold`, `Harness`, `Registry`, `Run`, out of `pkg/mtt`) → Task 2.
- Merge semantics (one serializer/R1, empty-as-absent/m4, nil-safe + refuse/m5, null-as-absent/r4, word-boundary
  presence/r5, idempotency) → Task 2 tests 1-15 equivalents.
- Atomic write / `internal/fsutil` + `SyncDir` + `audit.go` (M3/R2) → Task 1.
- `Run` semantics (write-as-you-go/R3, `os.IsNotExist`-only/r6) → Task 2 (`scaffold.go`, `run_test.go`).
- init failure contract (R3) → Task 4.
- No silent traps (opt-in, no-clobber, reported) → Tasks 3, 4 (`reportScaffold`, init contract).
- Docs (EN+RU, `internal/fsutil/CLAUDE.md`/D1, `internal/scaffold/CLAUDE.md`, stale-claim amendments, CHANGELOG)
  → Tasks 1, 2, 3, 5.

**Placeholder scan:** no TBD/TODO; every code step carries full code; the docs task steps are prose edits (no
code) and name exact files/lines.

**Type consistency:** `Action`/`Result`/`Harness`/`Registry`/`Run` (Task 2) are used verbatim by `agent.go`
and `init.go` (Tasks 3-4); `toScaffoldJSON`/`reportScaffold`/`scaffoldJSON` (Task 3) reused by init (Task 4);
`fsutil.AtomicWrite`/`SyncDir` (Task 1) used by `scaffold.Run` (Task 2) and the yaml adapter (Task 1).
`initJSON.Scaffold` is `[]scaffoldJSON`. Consistent.
