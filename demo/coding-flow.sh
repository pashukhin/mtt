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
# set -e catches any expected-PASS move that unexpectedly fails (so the guard test
# can't false-pass on a pass-path regression); expect_block's `|| rc=$?` keeps the
# INTENTIONAL blocking gates -e-safe.
set -eu

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
