# t4 — coding-template demo (runnable + tested) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `demo/coding-flow.sh` — a runnable, narrated end-to-end walkthrough of mtt's shipped `coding` template (feature/bugfix/refactor, each showing a gate BLOCK→pass) — guarded by a Go test in `make check` so it can't rot.

**Architecture:** A self-contained bash script builds a throwaway Go project in a temp dir, `mtt init --template coding`, and drives all three coding types through their gates using a freshly-built `mtt` (`${MTT_BIN:-mtt}`). A Go test builds mtt, runs the script, and asserts its stdout markers. No engine/domain/template change.

**Tech Stack:** bash, Go 1.23, `make`/`gofmt`/`git` (all already in CI), `mtt` CLI.

## Global Constraints

- Spec of record: `docs/superpowers/specs/t4-coding-flow-demo.md`.
- **Do NOT modify the `coding` template** or any config/engine/domain.
- **`make check` green before every commit.**
- The `coding` template declares **no `require`** → no attribution needed for moves.
- **Blocked gate = exit 3.** mtt's gate progress (`▶`/`✓`/`✗`) and block error go to **stderr**; move confirmations to stdout. The test asserts on the **script's own** narration markers.
- **Script hygiene (from the empirical spec review):** operate strictly inside a `mktemp -d` (trap-cleanup); `git config user.*` right after `git init`; **no bare `set -e`** (guard expected-fails); `unset MTT_DIR MTT_ROLE`; segment order **feature → bugfix → refactor**, each ending on a GREEN suite; use `mtt <status> <id>` sugar (transitions are unnamed, `mtt do` = exit 6).
- **`git diff --exit-code -- pkg/`** catches only **unstaged edits to tracked files** — commit a pkg/ baseline; the refactor's rejected change is an unstaged edit to a tracked `pkg/` file; the kept behavior-preserving change lives **outside** `pkg/`.
- Keep scaffold Go gofmt-clean by running **`gofmt -w .`** after writing Go (idempotent on clean files) — so `make lint` (gofmt-based) stays green and blocks are isolated to `make test` / `git diff`.
- Commit trailer: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`.

---

## Task 1: The demo script + Go test wrapper + Makefile target

**Files:**
- Create: `demo/doc.go`
- Create: `demo/coding-flow.sh` (executable)
- Create: `demo/coding_flow_test.go`
- Modify: `Makefile` (add a `demo` target)

**Interfaces:**
- Consumes: the built `mtt` binary via `${MTT_BIN:-mtt}`; the shipped `coding` template.
- Produces: `demo/coding-flow.sh` exiting 0 with stdout markers `feature: done`, `bugfix: done`, `refactor: done`, and exactly three `blocked as expected` lines.

- [ ] **Step 1: Create `demo/doc.go`.**

```go
// Package demo holds a runnable, tested end-to-end showcase of mtt's `coding`
// template (see coding-flow.sh). It has no importable API; coding_flow_test.go
// builds mtt and runs the script under `make check`.
package demo
```

- [ ] **Step 2: Write the Go test wrapper (it will fail — no script yet).**

Create `demo/coding_flow_test.go`:

```go
package demo

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestCodingFlowDemo builds mtt, runs demo/coding-flow.sh against it in a temp
// dir, and asserts the walkthrough reached `done` for each coding type and that
// each of the three deliberate gate blocks actually fired.
func TestCodingFlowDemo(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Dir(filepath.Dir(thisFile)) // .../demo -> repo root

	mttBin := filepath.Join(t.TempDir(), "mtt")
	build := exec.Command("go", "build", "-o", mttBin, "./cmd/mtt")
	build.Dir = repoRoot
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("build mtt: %v", err)
	}

	script := filepath.Join(repoRoot, "demo", "coding-flow.sh")
	cmd := exec.Command("bash", script)
	cmd.Env = append(os.Environ(),
		"MTT_BIN="+mttBin,
		"MTT_DIR=", "MTT_ROLE=", "MTT_BY=demo",
	)
	out, err := cmd.CombinedOutput()
	t.Logf("demo output:\n%s", out)
	if err != nil {
		t.Fatalf("demo script failed: %v", err)
	}

	got := string(out)
	for _, marker := range []string{"feature: done", "bugfix: done", "refactor: done"} {
		if !strings.Contains(got, marker) {
			t.Errorf("missing marker %q", marker)
		}
	}
	if n := strings.Count(got, "blocked as expected"); n != 3 {
		t.Errorf("want 3 blocked gates, got %d", n)
	}
}
```

- [ ] **Step 3: Run the test — expect FAIL (script missing).**

Run: `go test ./demo -run TestCodingFlowDemo -v`
Expected: FAIL — `demo script failed` (no `demo/coding-flow.sh`).

- [ ] **Step 4: Write the demo script.**

Create `demo/coding-flow.sh` (make it executable in Step 5):

```bash
#!/usr/bin/env bash
# demo/coding-flow.sh — a runnable, narrated showcase of mtt's `coding` template.
#
# Walks feature/bugfix/refactor end-to-end. Each type deliberately hits a gate
# that BLOCKS (exit 3), then passes — showing mtt as "a fuse between an agent and
# the word 'done'". The gates genuinely execute; nothing here is faked.
#
#   Run:  make demo            (builds mtt, runs this)
#    or:  ./demo/coding-flow.sh   (needs `mtt` on PATH, or set MTT_BIN)
#
# NOTE: no `set -e` — we intentionally trigger blocking gates and check for them.
set -u

MTT="${MTT_BIN:-mtt}"
unset MTT_DIR MTT_ROLE   # shell the real binary hermetically (nothing scrubs these for us)

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
cd "$work"

say()  { printf '\n\033[1m▶ %s\033[0m\n' "$*"; }
note() { printf '  %s\n' "$*"; }

# expect_block <mtt move...> : the move MUST be stopped by a gate (exit 3).
expect_block() {
  local rc=0
  "$@" || rc=$?
  if [ "$rc" -eq 0 ]; then echo "UNEXPECTED PASS: $*" >&2; exit 1; fi
  if [ "$rc" -ne 3 ]; then echo "expected exit 3, got $rc: $*" >&2; exit 1; fi
  note "blocked as expected (exit 3)"
}

# --- scaffold a tiny Go project the coding gates can actually run against ------
say "scaffold: a tiny Go project + coding template"
git init -q
git config user.email demo@example.com
git config user.name  "mtt demo"
printf 'module codingdemo\n\ngo 1.23\n' > go.mod
# Makefile (real tabs via printf): lint = fail on gofmt drift; test = go test.
printf 'lint:\n\t@out="$$(gofmt -l .)"; [ -z "$$out" ] || { echo "unformatted: $$out"; exit 1; }\ntest:\n\t@go test ./...\n' > Makefile
mkdir -p pkg/calc
cat > pkg/calc/calc.go <<'EOF'
package calc

// Add returns a + b.
func Add(a, b int) int { return a + b }

// Max returns the larger of a and b.
func Max(a, b int) int {
	if a > b {
		return a
	}
	return a // BUG: should return b
}
EOF
cat > pkg/calc/calc_test.go <<'EOF'
package calc

import "testing"

func TestAdd(t *testing.T) {
	if Add(2, 3) != 5 {
		t.Fatal("Add broken")
	}
}

// TestMax only checks the a>b case, so the bug in the else branch stays latent.
func TestMax(t *testing.T) {
	if Max(3, 1) != 3 {
		t.Fatal("Max broken")
	}
}
EOF
gofmt -w .
"$MTT" init --template coding >/dev/null
git add -A && git commit -qm "scaffold (green suite; latent Max bug)"

# =============================== FEATURE ======================================
# DoD = a green gate. A feature needs its test GREEN to finish.
say "feature: add Mul; done is BLOCKED while its test fails"
"$MTT" add "multiply helper" >/dev/null            # feature is the default type -> f1
"$MTT" in_progress f1 >/dev/null                    # tbd -> in_progress (no gate)
cat > pkg/calc/mul.go <<'EOF'
package calc

// Mul returns a * b.
func Mul(a, b int) int { return 0 } // not implemented yet
EOF
cat > pkg/calc/mul_test.go <<'EOF'
package calc

import "testing"

func TestMul(t *testing.T) {
	if Mul(2, 3) != 6 {
		t.Fatalf("Mul(2,3) = %d, want 6", Mul(2, 3))
	}
}
EOF
gofmt -w .
expect_block "$MTT" done f1                          # gate: make lint + make test (test RED) -> block
note "implement Mul, then done passes"
cat > pkg/calc/mul.go <<'EOF'
package calc

// Mul returns a * b.
func Mul(a, b int) int { return a * b }
EOF
gofmt -w .
"$MTT" done f1 >/dev/null
echo "feature: done"

# =============================== BUGFIX =======================================
# Red-first, enforced: a bugfix must have a FAILING repro test to even start.
say "bugfix: starting without a failing test is BLOCKED (! make test)"
"$MTT" add --type bugfix "Max returns wrong value when b > a" >/dev/null   # -> b1
expect_block "$MTT" in_progress b1                  # gate: ! make test (suite GREEN) -> block
note "write a failing repro test, then in_progress passes"
cat > pkg/calc/max_bug_test.go <<'EOF'
package calc

import "testing"

func TestMaxWhenBGreater(t *testing.T) {
	if Max(1, 3) != 3 {
		t.Fatalf("Max(1,3) = %d, want 3", Max(1, 3))
	}
}
EOF
gofmt -w .
"$MTT" in_progress b1 >/dev/null                    # ! make test (suite RED) -> pass
note "fix Max, then done passes"
cat > pkg/calc/calc.go <<'EOF'
package calc

// Add returns a + b.
func Add(a, b int) int { return a + b }

// Max returns the larger of a and b.
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
EOF
gofmt -w .
"$MTT" done b1 >/dev/null                            # make lint + make test (green) -> pass
echo "bugfix: done"

# commit everything so pkg/ is a clean baseline for the refactor's git-diff gate
git add -A && git commit -qm "feature Mul + bugfix Max landed"

# ============================== REFACTOR ======================================
# No public-API change: an uncommitted pkg/ edit is rejected by git diff -- pkg/.
say "refactor: a public pkg/ change is BLOCKED (git diff -- pkg/)"
"$MTT" add --type refactor "extract an internal helper; keep pkg/ API stable" >/dev/null  # -> r1
"$MTT" in_progress r1 >/dev/null                    # no gate
# the behavior-preserving refactor lives OUTSIDE pkg/ (a change under pkg/ would re-trip the gate)
mkdir -p internal/mathx
cat > internal/mathx/mathx.go <<'EOF'
package mathx

// Clamp returns v bounded to [lo, hi].
func Clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
EOF
# ...but we ALSO tweak the PUBLIC pkg/ surface (unstaged edit to a tracked file):
cat >> pkg/calc/calc.go <<'EOF'

// Double returns 2*a. (public API addition — the no-public-change gate rejects this)
func Double(a int) int { return 2 * a }
EOF
gofmt -w .
expect_block "$MTT" done r1                          # gate: git diff --exit-code -- pkg/ (dirty) -> block
note "drop the public change (revert pkg/), keep only the internal extraction"
git checkout -- pkg/calc/calc.go                     # pkg/ clean again
gofmt -w .
"$MTT" done r1 >/dev/null                            # git diff -- pkg/ clean + lint + test green -> pass
echo "refactor: done"

say "done — feature, bugfix, refactor all delivered through their gates"
```

- [ ] **Step 5: Make the script executable and add the `demo` Makefile target.**

Run: `chmod +x demo/coding-flow.sh`

In `Makefile`: (a) add `demo` to the existing `.PHONY` line (currently
`.PHONY: all build install smoke release test fmt fmt-check vet lint check tidy clean` → append ` demo`);
(b) add the target (after `build`). `BIN := bin/mtt` is **relative**, and the script `cd`s into a temp dir,
so pass an **absolute** path via `$(abspath $(BIN))`:

```make
demo: build
	@MTT_BIN=$(abspath $(BIN)) bash demo/coding-flow.sh
```

- [ ] **Step 6: Run the demo directly to confirm it works.**

Run: `make demo`
Expected: narrated output; three `blocked as expected (exit 3)` lines; ends with `refactor: done` then the closing banner; exit 0.

- [ ] **Step 7: Run the Go test — expect PASS.**

Run: `go test ./demo -run TestCodingFlowDemo -v`
Expected: PASS (markers present, exactly 3 blocks).

- [ ] **Step 8: Full gate.**

Run: `make check`
Expected: green (fmt + vet + lint + `go test -race -cover ./...` + build). The demo test builds mtt and shells out; it's heavier than a unit test but bounded.

- [ ] **Step 9: Commit.**

```bash
chmod +x demo/coding-flow.sh
git add demo/doc.go demo/coding-flow.sh demo/coding_flow_test.go Makefile
git commit -m "t4: runnable, tested coding-template demo (feature/bugfix/refactor gates)"
```

---

## Task 2: Docs — bilingual demo README + pointers

**Files:**
- Create: `demo/README.md`, `demo/README.ru.md`
- Modify: `README.md`, `README.ru.md` (the `## Docs` section)
- Modify: `DESIGN.md`, `DESIGN.ru.md` (the `mtt init --template coding` paragraph)

- [ ] **Step 1: Write `demo/README.md`.**

```markdown
# Coding-template demo

A runnable, tested walkthrough of mtt's `coding` starter template
(`mtt init --template coding` → `feature` / `bugfix` / `refactor`), each type
driven end-to-end through its **gated Definition of Done**.

## Run it

```sh
make demo            # builds mtt, runs the walkthrough
# or, with mtt on PATH:
./demo/coding-flow.sh
```

Everything happens in a throwaway temp dir — nothing touches your project.

## What it shows

Each type deliberately hits a gate that **blocks** (exit 3), then passes:

- **feature** — `done` is blocked while the test is red; a feature needs it **green to finish**.
- **bugfix** — starting is blocked until a **failing repro test** exists (`! make test`); a bugfix needs it **red to start**.
- **refactor** — `done` is blocked by a public `pkg/` change (`git diff --exit-code -- pkg/`); the refactor must keep the public API stable.

## Tested

`coding_flow_test.go` builds mtt, runs the script, and asserts each type reached
`done` and each block fired — so `make check` keeps this demo honest.
```

- [ ] **Step 2: Write `demo/README.ru.md`** (mirror, RU).

```markdown
# Демо coding-шаблона

Запускаемый и покрытый тестом проход по стартовому шаблону `coding`
(`mtt init --template coding` → `feature` / `bugfix` / `refactor`): каждый тип
проводится end-to-end через свой **гейтящий Definition of Done**.

## Запуск

```sh
make demo            # соберёт mtt и прогонит демо
# или, если mtt в PATH:
./demo/coding-flow.sh
```

Всё происходит во временном каталоге — ваш проект не затрагивается.

## Что показывает

Каждый тип намеренно упирается в гейт, который **блокирует** (exit 3), затем проходит:

- **feature** — `done` заблокирован, пока тест красный; фиче нужен **зелёный, чтобы завершиться**.
- **bugfix** — старт заблокирован, пока нет **падающего репро-теста** (`! make test`); багфиксу нужен **красный, чтобы начать**.
- **refactor** — `done` заблокирован публичным изменением `pkg/` (`git diff --exit-code -- pkg/`); рефакторинг обязан сохранить публичный API.

## Тест

`coding_flow_test.go` собирает mtt, гоняет скрипт и проверяет, что каждый тип дошёл
до `done` и каждый блок сработал — так `make check` держит демо честным.
```

- [ ] **Step 3: Add the demo pointer to README `## Docs` (EN + RU).**

Read the `## Docs` section of `README.md`; add a bullet (match the existing list style), e.g.:
`- [Coding-template demo](demo/README.md) — a runnable, tested `coding`-flow walkthrough.`
Mirror in `README.ru.md` — its heading is **`## Документация`** (not `## Docs`):
`- [Демо coding-шаблона](demo/README.ru.md) — запускаемый проход по потоку `coding`.`

- [ ] **Step 4: Add a demo pointer in DESIGN (EN + RU).**

Grep the coding-init paragraph — note the phrase has a backtick (`` `mtt init --template coding` ships … ``), so grep a break-free substring: `grep -n 'ships example coding types' DESIGN.md` and `grep -n 'coding-типы' DESIGN.ru.md` (paragraph is at DESIGN.md:679 / DESIGN.ru.md:687). Append a sentence to that paragraph in both files, e.g.:
- EN: `See `demo/` for a runnable, tested end-to-end walkthrough of this template.`
- RU: `Запускаемый end-to-end проход по этому шаблону — в `demo/`.`

- [ ] **Step 5: Gate + commit.**

Run: `make check`
Expected: green.

```bash
git add demo/README.md demo/README.ru.md README.md README.ru.md DESIGN.md DESIGN.ru.md
git commit -m "t4: docs — bilingual demo README + README/DESIGN pointers"
```

---

## Final verification (before `mtt submit` → impl_review)

- [ ] `make check` green from a clean tree.
- [ ] `make demo` runs green, showing three blocks then all-`done`.
- [ ] Principles self-check (AGENTS.md): clean architecture (no engine/template change — demo is an external artifact), KISS (one script, one guard test), DRY (single script is the source of truth), TDD (the guard test written before the script, red→green).
- [ ] Docs-sync judgment: `demo/README.md`↔`.ru.md` and README/DESIGN pointers in EN+RU lockstep.
