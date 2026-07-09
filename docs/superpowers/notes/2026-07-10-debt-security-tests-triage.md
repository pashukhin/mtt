# Design-debt triage, security review, test-suite audit — findings (vectors 4/6/7)

Status: **analysis findings + recommendations**, 2026-07-10. Third and final note of the deep-analysis
session; companions:
[2026-07-09-positioning-and-agent-ux-analysis.md](2026-07-09-positioning-and-agent-ux-analysis.md)
(positioning + agent UX, `R*`/`U*`) and
[2026-07-09-s009-readiness-and-architecture-audit.md](2026-07-09-s009-readiness-and-architecture-audit.md)
(s009 readiness + architecture audit, `S*`/`A*`). This note records the **backlog triage** (with the
scale measurements), the **security/trust review** (`SEC*`), and the **test-suite audit** (`T*`), plus a
disposition table saying where each item is scheduled (or deliberately not).

---

## Part A — design-debt & parked-decisions triage

Overall: the deferred backlog is honest — almost everything parked has a correctly-recorded trigger.
Actions taken 2026-07-10 (already applied to TASKS.md): the shipped "show description on move" item
marked SHIPPED (s008.95); the scale think-item updated with measurements (below); a new
**lost-update / write-concurrency** think-item added (audit A3); **dangerous-ops attribution elevated**
(first candidate once s009 lands — `--no-run` becomes the legal bypass of a red `make check`);
`ErrUnsupported` scheduled pre-`v0.9.0` (audit A7, → chore s009.5).

### Scale measurement (the think-item's "cheap first pass" — done)

Method: generated flat labs of N tasks (`.mtt/tasks/tN.yaml`, every 5th with one `depends_on` edge),
default template config, timed with warm FS cache, binary `0.8.9-dev`:

| command | N=100 | N=1000 | N=5000 | scaling |
|---|---|---|---|---|
| `list` / `ready` / `tree` / `dep list` / `show` | 6–18 ms | 24–35 ms | ~120 ms | linear (I/O-bound) — fine |
| `status` (gated transition) | 3 ms | 4 ms | 3 ms | O(1) — gate path independent of N |
| `roadmap` | 8 ms | 55 ms | **1.02 s** | **~quadratic** |
| `dep add` (cycle check, graph rebuild) | 7 ms | 26 ms | 113 ms | linear — fine |

Diagnosis: the `roadmap` hotspot is **not** I/O and not the graph build — it is the documented Kahn
"re-sort the available set before each pop" (`sort.SliceStable` per emitted node ⇒ O(N²·log N) when most
nodes are independent). Extrapolation: ~4–5 s at N=10k. **Harmless at dogfood volume** (10² tasks ⇒
sub-ms). Fix when needed: a heap (`container/heap` keyed by `(effective rank, recency)`) replacing the
per-pop sort. The original suspicion — "the dominant cost is likely I/O" — is retired by measurement.
The graph-*topology* stress half (dense flows, deep chains, fan-out) remains open, still best after
`advance`/`ResolvedFlow` unparks.

### Triage verdicts on the parked items (kept parked — with watch-signals)

- **`advance`/`start`/`done` + modes + roles-on-edges** — parking holds: each move in the full
  decision-A flow is deliberate and gated, single-edge stays the norm. **Watch-signal (new):**
  decision A stretches a session from 2 to 5 edges — the first real pressure source on `advance`. If
  after 2–3 self-hosted sessions walking `speccing → planning → …` feels mechanical, the unpark trigger
  has fired. Roles still correctly wait on subagent identity.
- **Subagent identity / per-agent `current` / multi-assignee** — one multi-agent cluster together with
  the new lost-update item (A3) and monotonic minting (A4): all trigger at "the second writing agent on
  one store". Decide them together, not piecemeal.
- **`cancelled`-blocker semantics** — dogfood will produce the first live data (a cancelled session that
  others `depends_on`). Reassess after a few self-hosted sessions, not before.
- **Node-level status actions** — YAGNI confirmed by the project's own flow-granularity insight #1
  (progressing work is statuses, not verbs); agree with the parking.
- **Argument-resolution grammar** — correctly gated on "before more sugar forms"; none planned through
  s012. Revisit if custom verbs / node actions ever unpark.
- **`list` filter-value validation** — cost of the silent-empty rises with self-hosting (a typo'd
  `--status speccing` vs a 6-status flow returns an empty list indistinguishable from "no tasks"). Cheap;
  candidate for the tail of s009 or s010.
- **Graphviz flow export** — unchanged priority, but noted as a cheap onboarding nicety once dogfood
  makes "show me the flow" a first-contact question.
- Everything else (boards/views, KB, Gantt, re-parenting, edit-audit, `--editor`, bulk
  status/verbs/edit/dep) — phase queue, no pressure, no change.

---

## Part B — security / trust-model review (`SEC*`)

Verdict: **the trust model is coherent and honestly documented; no real holes found.** The risk class is
"config-as-code" (same as a Makefile), and the project states it explicitly (exec package doc: "trusted
project config, like a Makefile, never network input").

### Verified and holding

- **Placeholder injection defense is structural and works:** `core/expand.go` exposes exactly
  `{ID, Type, From, To}`; free text (title/description) cannot reach a shell command by construction —
  `{{.Title}}` is a template *error*, not an empty substitution. In a shared repo a task title IS
  attacker-influenced (anyone can add a task via PR) and it never reaches the shell. Holds.
- **Zero-network binary:** no `net`/`http` anywhere in the shipped packages; 3 direct deps (cobra,
  `yaml.v3` v3.0.1 — the DoS-CVE-fixed version, go-internal), 4 indirect. Minimal supply-chain surface.
- **Secrets discipline:** credentials only in gitignored `config.local.yaml`/env (documented); gates
  inherit the parent env, but that is inside the trust boundary.
- **CI/release:** `release.yml` has minimal `permissions: contents: write`, built-in `gh`, no
  third-party actions; `SHA256SUMS` shipped.
- `--log-file` is a plain same-privilege `os.Create`; `init` refuses to overwrite without `--force`.

### Findings (operational nuances, not holes)

- **SEC1 — timeout does not kill the process group.** `exec.CommandContext` kills `sh` at the deadline,
  but grandchildren holding the inherited output pipe keep `Run()` blocked until they exit (the project
  already observes this in tests). A gate that accidentally spawns a daemon hangs mtt past its timeout.
  Fix when wanted: `SysProcAttr{Setpgid: true}` + kill the process group on Unix. Backlog think-item,
  low priority solo.
- **SEC2 — self-referential gates should be read-only.** The s009 §4 pattern (a gate shelling out to
  `mtt list …`) is safe; a gate invoking an mtt **transition** could recurse (transition → gate →
  transition …), bounded only by timeouts. One caution line belongs in the s009 config docs / spec
  reconciliation (fold into S1): "gates may invoke read-only mtt commands only."
- **SEC3 — Windows gate path is released but never CI-tested** (`cmd /c` branch; ubuntu-only CI;
  release cross-compiles without running tests). Honesty line in README/RELEASING — scheduled into
  chore s009.5.
- **SEC4 — `SHA256SUMS` is integrity, not authenticity** (the sums live next to the binaries; a
  compromised release compromises both). Artifact signing / `gh attestation` — later, explicitly not
  for `v0.9.0`.
- **Config-review-as-code doc line** — "review `.mtt/config.yaml` diffs like Makefile diffs" (especially
  for agent reviewers, where a YAML diff reads as data) — scheduled into chore s009.5. Related mechanism
  note: `FindRoot` walks up, so running mtt under an untrusted parent directory picks up that tree's
  config (demonstrated by the stray `bin/.mtt`); same trust boundary, no code change.
- `--no-run` as the legal gate bypass — already elevated (TASKS dangerous-ops item).

---

## Part C — test-suite audit (`T*`)

Verdict: **the gate is real and coverage is strong; the "table-driven" claim is overstated; the riskiest
untested paths are exactly the audit's A-items.**

### Coverage (go test -cover, all green)

`pkg/mtt` 97.6% · `internal/core` 95.6% · `internal/adapter/exec` 92.9% · `internal/cli` 87.0% ·
`internal/adapter/yaml` 83.8% · `cmd/mtt` 0% (2-line main). Worst-covered functions:
`cli/dep.go writeDepCycles` 25%, `cli/dep.go buildDepListJSON` 36%, `yaml/current.go documentRoot` 43%,
`cli/bulk.go previewBulk` 44%, `yaml/init.go atomicWrite` 47%.

### Claim verdicts

| claim (AGENTS.md) | verdict | evidence |
|---|---|---|
| TDD every feature | UNVERIFIABLE (squash merges destroy red→green ordering); proxy evidence consistent | every feature has unit + e2e |
| table-driven unit tests in core/yaml | **PARTIAL — overstated**: dominant core style is scenario-per-function (add/remove/edit/ready/tags: 0 tables) | either adopt tables or soften the AGENTS.md wording |
| golden YAML + `-update` | HOLDS (5 goldens lock field order, omitempty, timestamps; flag exists) | no golden contains `history` (round-trip only) |
| e2e testscript in temp dirs | HOLDS — 17 scripts, near-complete command matrix | gaps below |
| no network in tests | HOLDS | grep clean |
| `make check` = CI | HOLDS | ci.yml mirrors Makefile |

Also: `-race` is applied but substantively vacuous — the one real concurrency surface (`mint` O_EXCL
contention) has no concurrent test; exec timeout tests use real sleeps (acceptable 50× margins).

### e2e gaps (commands/flags with no e2e)

`-v/--verbose` (zero occurrences anywhere — the stderr-streaming branch is fully untested), `version`
(unit-only), `completion` (zero tests), `--json` on a status move, `--role`/`MTT_BY` env end-to-end.
Exit codes: the numeric mapping is unit-tested and messages are e2e-tested, but no test asserts
`Execute()`'s returned int through a full blocked/attribution run (testscript can only assert non-zero).
Note: `structured_commands.txt` skips **wholesale** without git, silently dropping the per-command
timeout and scalar back-compat e2e on gitless environments.

### Top missing tests, ranked (risk ÷ effort), with disposition

| # | test | disposition |
|---|---|---|
| T1 | corrupt + zero-byte task file → `Get`/`List` error names the file | **scheduled — chore s008.97** (with A1) |
| T2 | invalid committed config: `add`/`types` reject; lock what `status`/`list` do (today: silently proceed — the Load-doesn't-Validate guard) | unscheduled — pairs with S6; candidate for s008.97 or s009 |
| T3 | `rm --dry-run --json` / `previewBulk` JSON branch (44%) | unscheduled — one script line, any session |
| T4 | `writeDepCycles` with a real cycle (25%; reachable only via hand-edited files — exactly when it must work) | unscheduled — unit test, any session |
| T5 | stale `current` in bare-verb sugar (`root.go`) and `mtt use` | unscheduled — pairs with A5's exit-code decision (s009.5) |
| T6 | split per-command-timeout e2e out of the git-gated script | **scheduled — chore s008.97** |
| T7 | `-v` gate streaming | **scheduled — chore s008.97** |
| T8 | full-stack exit-code assertions (2/3/6 composition via `Execute()`, pattern exists in `TestRmMissingTaskExit4`) | unscheduled — cheap unit batch, any session |
| — | concurrent `mint` (O_EXCL) test; `atomicWrite` failure modes | unscheduled — join the A3/A4 "concurrent store semantics" brainstorm |

---

## Disposition summary (this note's items only)

| when | items |
|---|---|
| chore s008.97 | T1 (=A1), T6, T7 |
| s009 (spec reconciliation) | SEC2 caution line |
| chore s009.5 | SEC3, config-review-as-code line, (A5 brings T5 along) |
| backlog / think-items | SEC1 (process-group kill), SEC4 (signing — post-v0.9.0), T2–T5, T8, concurrent-mint, roadmap heap (with the scale item), AGENTS.md "table-driven" wording decision |
| already applied to TASKS.md (2026-07-10) | shipped-marker for guidance item; scale measurements; lost-update think-item; dangerous-ops elevation; ErrUnsupported → s009.5 |
