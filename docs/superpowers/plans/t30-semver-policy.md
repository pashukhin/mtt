# t30 — SemVer + tag-as-SoT Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the session-mirrored `0.N.M-dev` version scheme with SemVer whose single source of truth is the git tag: the version is derived at build/run time, no number is hand-maintained in source, and the docs describe the SemVer bump-from-changelog process.

**Architecture:** A pure `resolve(ldflags, buildVersionFn)` core in `internal/cli/version.go` picks the effective version (ldflags-injected value → `runtime/debug` build-info module version → `"dev"`); cobra's `Version:` field and the `mtt version` command both call it. The Makefile derives a `git describe` version for **dev** build targets only, while `release` keeps requiring an explicit `VERSION` (its guard is preserved). Five living docs are reconciled to state the new policy positively.

**Tech Stack:** Go 1.x (cobra CLI), `runtime/debug.ReadBuildInfo`, GNU Make, Markdown docs (EN + RU).

## Global Constraints

- Spec (authority): `docs/superpowers/specs/t30-semver-policy.md`. Decisions D1–D5 are binding.
- TDD: red → green → refactor. `make check` (gofmt + vet + golangci-lint v2 + `go test -race -cover` + build) MUST be green before every commit.
- Stay on the **0.x** line; do NOT cut 1.0.
- The git tag `vX.Y.Z` is the single source of truth; **no version number is committed to source** (the ldflags target `version` defaults to `"dev"`, not a number).
- `release` MUST still error without an explicit `VERSION`; a `git describe` string must never become a release stamp.
- ldflags symbol path is exactly `github.com/pashukhin/mtt/internal/cli.version` (do not rename the `version` package var).
- Docs language rule: EN is source of truth; `DESIGN.ru.md` kept in sync with `DESIGN.md`. Do NOT edit dated/superseded artifacts under `docs/superpowers/plans|specs/*` or `TASKS.md` (frozen history). `NEXT_SESSION.md` is out of scope (D5).
- Non-`.mtt` changes are committed by hand on branch `task/t30` (mtt post-hooks commit only `.mtt/`).

---

### Task 1: Version resolution (`resolve`/`resolveVersion`) + wire-up

**Files:**
- Modify: `internal/cli/version.go` (add the `version` var + resolvers; print `resolveVersion()`)
- Modify: `internal/cli/root.go:15-16,31` (remove the `version` var; `Version: resolveVersion()`)
- Modify: `internal/cli/root_test.go:112-127` (assert against `resolveVersion()`)
- Test: `internal/cli/version_test.go` (new — unit tests for `resolve`)

**Interfaces:**
- Produces: `resolve(ldflags string, buildVersion func() string) string` (pure); `resolveVersion() string`; `readBuildVersion() string`; package var `var version = "dev"`.
- Consumes: nothing from other tasks.

- [ ] **Step 1: Write the failing unit test.** Create `internal/cli/version_test.go`:

```go
package cli

import "testing"

func TestResolve(t *testing.T) {
	tests := []struct {
		name         string
		ldflags      string
		buildVersion string
		want         string
	}{
		{"ldflags value wins", "v0.9.0", "", "v0.9.0"},
		{"ldflags wins over build info", "v0.9.0", "v1.2.3", "v0.9.0"},
		{"build info when ldflags is dev", "dev", "v1.2.3", "v1.2.3"},
		{"fallback on (devel) build info", "dev", "(devel)", "dev"},
		{"fallback on empty build info", "dev", "", "dev"},
		{"fallback on empty ldflags", "", "", "dev"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolve(tc.ldflags, func() string { return tc.buildVersion })
			if got != tc.want {
				t.Fatalf("resolve(%q, ()->%q) = %q, want %q", tc.ldflags, tc.buildVersion, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run it, verify it fails to compile.**

Run: `go test ./internal/cli/ -run TestResolve`
Expected: FAIL — `undefined: resolve`.

- [ ] **Step 3: Implement the resolvers.** Replace the whole body of `internal/cli/version.go` with:

```go
package cli

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// version is the ldflags-injected build version ("-X …/internal/cli.version=…").
// It defaults to "dev"; release and explicit `make` builds stamp a real value.
var version = "dev"

// resolveVersion returns the effective version: an ldflags-injected value wins,
// else the module version recorded in the build info (populated by
// `go install …@vX.Y.Z`), else "dev".
func resolveVersion() string {
	return resolve(version, readBuildVersion)
}

// resolve is the testable core of version resolution.
func resolve(ldflags string, buildVersion func() string) string {
	if ldflags != "" && ldflags != "dev" {
		return ldflags
	}
	if bv := buildVersion(); bv != "" && bv != "(devel)" {
		return bv
	}
	return "dev"
}

// readBuildVersion returns the main module's version from the build info, or ""
// when it is unavailable (plain `go build`, `go test`).
func readBuildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.Main.Version
	}
	return ""
}

// newVersionCmd prints the build version.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the mtt version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), resolveVersion())
			return err
		},
	}
}
```

- [ ] **Step 4: Remove the old `version` var from `root.go` and wire the resolver.** In `internal/cli/root.go` delete these two lines (15-16):

```go
// version is the build version, overridable at build time via -ldflags.
var version = "0.9.0-dev"
```

and change line 31 from `Version:       version,` to:

```go
		Version:       resolveVersion(),
```

- [ ] **Step 5: Update `TestVersionCommand`.** In `internal/cli/root_test.go`, replace lines **122-125** (the comment line included, so it is not duplicated):

```go
	// version prints to STDOUT (not stderr), so an agent can capture it.
	if got := strings.TrimSpace(outBuf.String()); got != resolveVersion() {
		t.Fatalf("version stdout = %q, want %q", got, resolveVersion())
	}
```

- [ ] **Step 6: Run the tests, verify green.**

Run: `go test ./internal/cli/ -run 'TestResolve|TestVersionCommand' -v`
Expected: PASS (6 `TestResolve` subtests + `TestVersionCommand`). In test context `version=="dev"` and build-info version is `""`/`"(devel)"`, so `mtt version` prints `dev`.

- [ ] **Step 7: Full gate.**

Run: `make check`
Expected: `OK: make check passed`.

- [ ] **Step 8: Commit.**

```bash
git add internal/cli/version.go internal/cli/root.go internal/cli/version_test.go internal/cli/root_test.go
git commit -m "t30: derive version (ldflags -> build info -> dev); drop the 0.9.0-dev literal"
```

---

### Task 2: Makefile — git-describe for dev builds, explicit VERSION for release

**Files:**
- Modify: `Makefile:4-8` (version variables) and the `build` / `install` / `smoke` / `release` recipes.

**Interfaces:**
- Consumes: the `version` ldflags symbol from Task 1.
- Produces: `BUILD_LDFLAGS` (dev) and `RELEASE_LDFLAGS` (release) make variables.

- [ ] **Step 1: Replace the version-variable block.** In `Makefile`, replace lines 4-8:

```make
VERSION ?=

ifneq ($(VERSION),)
LDFLAGS := -ldflags "-X github.com/pashukhin/mtt/internal/cli.version=$(VERSION)"
endif
```

with:

```make
VERSION ?=

# Dev builds derive the version from git when VERSION is not passed; release must
# always be stamped with an explicit VERSION (see the release target's guard).
BUILD_VERSION := $(if $(VERSION),$(VERSION),$(shell git describe --tags --always --dirty 2>/dev/null))
BUILD_LDFLAGS := $(if $(BUILD_VERSION),-ldflags "-X github.com/pashukhin/mtt/internal/cli.version=$(BUILD_VERSION)")
RELEASE_LDFLAGS := -ldflags "-X github.com/pashukhin/mtt/internal/cli.version=$(VERSION)"
```

- [ ] **Step 2: Point dev recipes at `BUILD_LDFLAGS`.** Change `build` (line 18), `install` (line 21), and `smoke` (line 29) to use `$(BUILD_LDFLAGS)` in place of `$(LDFLAGS)`:

```make
build:
	$(GO) build $(BUILD_LDFLAGS) -o $(BIN) ./cmd/mtt

install:
	$(GO) install $(BUILD_LDFLAGS) ./cmd/mtt
```

and in the `smoke` recipe: `GOBIN=$$tmp $(GO) install $(BUILD_LDFLAGS) ./cmd/mtt; \`

- [ ] **Step 3: Point the release recipe at `RELEASE_LDFLAGS`.** In the `release` recipe, change the per-platform build line (currently line 48) to use `$(RELEASE_LDFLAGS)`:

```make
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 $(GO) build $(RELEASE_LDFLAGS) -o "$$out" ./cmd/mtt; \
```

Leave the `[ -z "$(VERSION)" ]` guard at lines 40-41 untouched.

- [ ] **Step 4: Verify a bare dev build stamps a describe string.**

Run: `make build && ./bin/mtt version`
Expected: a non-empty git-describe string — a short SHA (e.g. `4c1c5f7`) while no `v*` tag exists, or `v0.9.0-3-gsha`/`…-dirty` after tagging. NOT `dev`, NOT `0.9.0-dev`.

- [ ] **Step 5: Verify the release guard still fires without VERSION.**

Run: `make release; echo "exit=$?"`
Expected: prints `release: VERSION is required (e.g. make release VERSION=v0.9.0)` and a non-zero exit. It must NOT build anything into `dist/`.

- [ ] **Step 6: Verify an explicit release stamps the tag (build only, no publish).**

Run: `make release VERSION=v0.9.0 && ./dist/mtt_v0.9.0_linux_amd64 version` (adjust the arch suffix to your host)
Expected: prints `v0.9.0`. Then clean up: `rm -rf dist`.

- [ ] **Step 7: Smoke + gate.**

Run: `make smoke && make check`
Expected: `OK: smoke (mtt version = <describe>)` then `OK: make check passed`.

- [ ] **Step 8: Commit.**

```bash
git add Makefile
git commit -m "t30: Makefile — git-describe for dev builds; keep release on explicit VERSION"
```

---

### Task 3: Reconcile the living docs to the SemVer/tag policy

**Files:**
- Modify: `RELEASING.md` (step 1, the pre-1.0 paragraph, add a policy section)
- Modify: `CHANGELOG.md` (header + an `[Unreleased]` entry)
- Modify: `DESIGN.md:963` and `DESIGN.ru.md:979`
- Modify: `sessions/README.md:15-17`

**Interfaces:**
- Consumes: the behavior from Tasks 1–2 (tag-as-SoT, no source version). No code.
- Produces: docs that satisfy acceptance criterion 1 (bilingual grep clean).

- [ ] **Step 1: `RELEASING.md` — rewrite step 1.** Replace the current step 1 (lines 9-10):

```markdown
1. **Bump the dev version.** Set `version` in [internal/cli/root.go](internal/cli/root.go) to the next
   `-dev` value if it hasn't been already (e.g. `0.9.0-dev`). This is what unreleased builds report.
```

with:

```markdown
1. **Pick the bump.** The version *is* the git tag — no source edit. Choose `X.Y.Z` from the accumulated
   `[Unreleased]` CHANGELOG categories per the **Versioning policy** below. Unreleased builds report a
   `git describe` string; `go install …@vX.Y.Z` reports the module version.
```

- [ ] **Step 2: `RELEASING.md` — replace the pre-1.0 paragraph with a policy section.** Replace the trailing lines 30-31:

```markdown
Pre-1.0 versions mirror the session (see [sessions/README.md](sessions/README.md)): a full session bumps the
minor, a point-session the patch.
```

with:

```markdown
## Versioning policy

mtt follows [Semantic Versioning](https://semver.org). The annotated git tag `vX.Y.Z` is the single source
of truth; the version is derived at build time (ldflags / `git describe`) and at run time from the module
build info — nothing is hand-maintained in source.

**Pre-1.0 (0.y.z):** bump **MINOR** (`0.→Y←.0`) for any new feature and/or any backward-incompatible change;
bump **PATCH** (`0.y.→Z←`) for backward-compatible fixes, security fixes, docs, and internal changes. A
breaking change is never shipped as a PATCH — it forces a MINOR and a `Changed`/`Removed` CHANGELOG entry
with a migration note.

**The compat surface** SemVer governs: the CLI (commands, flags, positional grammar, exit codes, and the
`MTT_DIR`/`MTT_BY`/`MTT_ROLE` env vars), the `--json` output schema, and the `.mtt` store schema *and its
semantics* (the `{{.ID}}`/`{{.Type}}`/`{{.From}}`/`{{.To}}` placeholder vocabulary, the exit-code/gate-block
contract, the `require:` keys). The public `pkg/mtt` Go API is best-effort pre-1.0.

**Bump from the changelog (at release cut):** any `Added`/`Changed`/`Deprecated`/`Removed` in `[Unreleased]`
→ MINOR; only `Fixed`/`Security`/docs/internal → PATCH.
```

- [ ] **Step 3: `CHANGELOG.md` — fix the header.** Replace lines 3-6:

```markdown
All notable changes to mtt are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Pre-1.0 versions mirror the development
session (see [sessions/README.md](sessions/README.md)): a full session bumps the minor, a point-session
the patch.
```

with:

```markdown
All notable changes to mtt are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the version scheme is
[Semantic Versioning](https://semver.org); see [RELEASING.md](RELEASING.md) for the pre-1.0 bump rules.
```

- [ ] **Step 4: `CHANGELOG.md` — add an `[Unreleased]` entry.** Under the `## [Unreleased]` heading, add a `### Changed` bullet (create the `### Changed` subheading if the Unreleased block doesn't already have one):

```markdown
### Changed
- **Versioning:** adopted SemVer (pre-1.0); the version is now derived from the git tag
  (ldflags / `git describe` → module build info → `"dev"`), replacing the hand-maintained session-mirrored
  `0.N.M-dev` literal. See [RELEASING.md](RELEASING.md).
```

- [ ] **Step 5: `DESIGN.md:963` — replace the session sentence.** Replace:

```markdown
workflow, `SHA256SUMS`) lives in [RELEASING.md](RELEASING.md). Pre-1.0 versions mirror the session.
```

with:

```markdown
workflow, `SHA256SUMS`) lives in [RELEASING.md](RELEASING.md). The version is derived from the git tag (SemVer).
```

- [ ] **Step 6: `DESIGN.ru.md:979` — replace the RU equivalent.** Replace:

```markdown
описаны в [RELEASING.md](RELEASING.md). Версии до 1.0 зеркалят сессию.
```

with:

```markdown
описаны в [RELEASING.md](RELEASING.md). Версия выводится из git-тега (SemVer; см. [RELEASING.md](RELEASING.md)).
```

- [ ] **Step 7: `sessions/README.md:15-17` — drop the version mnemonic.** Replace the bullet:

```markdown
- **Version mirrors the session** (pre-1.0 mnemonic, not strict semver): `sN` → `0.N.0-dev`, a
  point-session `sN.M` → `0.N.M-dev` (a full session bumps the minor, a point-session the patch). E.g.
  s006 → `0.6.0-dev`, s006.5 → `0.6.5-dev`, s006.7 → `0.6.7-dev`, s007 → `0.7.0-dev`.
```

with:

```markdown
- **Versioning:** SemVer, derived from the git tag — see [RELEASING.md](../RELEASING.md).
```

- [ ] **Step 8: Run the bilingual acceptance grep (must be clean).**

Run:
```bash
grep -rniE "mirror.*session|session.*(minor|patch)|point-session|зеркал.*сесс|сесси.*(минор|патч)|полная сессия" \
  RELEASING.md CHANGELOG.md DESIGN.md DESIGN.ru.md sessions/README.md
```
Expected: no matches (exit 1). If anything matches, it is stating the retired rule — rephrase positively.

- [ ] **Step 9: Gate (docs-only, but keep it green).**

Run: `make check`
Expected: `OK: make check passed`.

- [ ] **Step 10: Commit.**

```bash
git add RELEASING.md CHANGELOG.md DESIGN.md DESIGN.ru.md sessions/README.md
git commit -m "t30: docs — SemVer/tag-as-SoT policy; retire the session-mirror rule (EN+RU)"
```

---

## Final acceptance (run after all tasks)

- [ ] **AC-1:** the bilingual grep from Task 3 Step 8 returns nothing; `RELEASING.md` has the Versioning policy section; `NEXT_SESSION.md` untouched.
- [ ] **AC-2:** `grep -n '0\.9\.0-dev\|0\.N\.M' internal/cli/root.go` returns nothing; `go test ./internal/cli/ -run TestResolve -v` shows all three branches (ldflags / build-info / dev) passing.
- [ ] **AC-3:** `make build && ./bin/mtt version` prints a describe string; `make release` (no VERSION) errors non-zero; `make release VERSION=v0.9.0` stamps `v0.9.0`; `mtt --version` and `mtt version` agree; `make smoke` green. (Clean up `dist/` and `bin/` after.)
- [ ] **AC-4:** `make check` green.

## Self-review notes

- **Spec coverage:** D1/D2/D4 → Task 3 Step 2 (policy section). D3 code → Task 1; D3 Makefile → Task 2. D5 → Task 3 Steps 1-7. Acceptance criteria → Final acceptance.
- **No new version literal** is introduced in source (Task 1 Step 4 removes `0.9.0-dev`; the only `v0.9.0` occurrences are example commands in docs/tests, not a committed source-of-truth).
- **Type consistency:** `resolve`/`resolveVersion`/`readBuildVersion` names are identical across Task 1 and the tests; `BUILD_LDFLAGS`/`RELEASE_LDFLAGS` identical across Task 2 recipes.
