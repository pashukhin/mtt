# t31 — Retire the pre-mtt session apparatus: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Execute the approved spec `docs/superpowers/specs/t31-retire-pre-mtt-apparatus.md` — seed the
KB (12 notes), mechanize submit/approve gates + a CHANGELOG gate, purge the pre-mtt apparatus files,
rewire pointers, codify mtt-first rules.

**Architecture:** No production Go code changes. Four surfaces: `internal/adapter/yaml/dogfood_test.go`
(guard test, changed FIRST — TDD on config), `.mtt/config.yaml` (gates/descriptions), `.mtt/knowledge/`
(via `mtt note add` only — never hand-written files), and markdown docs (delete + rewire + rules).

**Tech stack:** Go test, YAML flow config, `./bin/mtt` CLI (v0.10.0-17 works), git.

## Global constraints

- Branch: `task/t31` (already on it). **t31 is in `implementing` while this plan executes** — every
  `mtt submit t31` below exercises `implementing→impl_review` with the FULL new gate stack
  (clean-tree → CHANGELOG → make check), and a probe run on a fully-committed tree without the
  scratch file would REALLY advance the task. Every commit leaves `make check` green.
- Config command scalars are **single-quoted YAML**; descriptions are double-quoted (existing style).
- Commit non-`.mtt` work before any `mtt submit`/`approve` — after Task 2 this is machine-enforced.
- Note bodies: markdown but **no `#` headings** (future-proof vs t49 hashtag extraction) — paragraphs
  and `-` bullets only. No note duplicates DESIGN/AGENTS content (pointers allowed).
- Commit trailer: `Co-Authored-By: <acting model name> <noreply@anthropic.com>`.

---

### Task 1: Guard test first (red)

**Files:** Modify: `internal/adapter/yaml/dogfood_test.go`

- [ ] **Step 1.1: Add the two new command constants** to the `const` block (after `cmdMakeCheck`,
  line ~20). Byte-for-byte the strings that Task 2 puts in the config:

```go
	cmdCleanTree = `out=$(git status --porcelain -- ":(exclude).mtt") && test -z "$out" || { echo "working tree not clean - commit your code/docs first (.mtt is swept by the move itself)" >&2; false; }`
	cmdChangelog = `git diff --quiet main...HEAD -- cmd internal pkg go.mod go.sum || git diff --name-only main...HEAD -- CHANGELOG.md | grep -q . || { echo "code changed but CHANGELOG.md has no entry - add one under [Unreleased] (pure refactor? bypass: mtt do submit --no-run --who ... --why ...)" >&2; false; }`
```

- [ ] **Step 1.2: spec/plan submit assertions** (lines ~165-175) — each edge gains the clean-tree gate
  as command #2:

```go
	for _, sp := range [][2]mtt.StatusName{
		{"speccing", "spec_review"}, {"spec_fix", "spec_review"},
	} {
		assertRuns(t, task, namedEdge(t, task, sp[0], sp[1], "submit"), cmdSpecGlob, cmdCleanTree)
	}
	for _, sp := range [][2]mtt.StatusName{
		{"planning", "plan_review"}, {"plan_fix", "plan_review"},
	} {
		assertRuns(t, task, namedEdge(t, task, sp[0], sp[1], "submit"), cmdPlanGlob, cmdCleanTree)
	}
```

- [ ] **Step 1.3: impl submit assertions** (lines ~177-188) — order clean-tree → changelog →
  `make check`; the 10m timeout moves to `Commands[2]`:

```go
	for _, tc := range []mtt.Type{task, chore} {
		for _, from := range []mtt.StatusName{"implementing", "impl_fix"} {
			e := namedEdge(t, tc, from, "impl_review", "submit")
			assertRuns(t, tc, e, cmdCleanTree, cmdChangelog, cmdMakeCheck)
			if e.Commands[2].Timeout != 10*time.Minute {
				t.Fatalf("%s %s->impl_review make-check timeout = %v, want 10m", tc.Name, from, e.Commands[2].Timeout)
			}
		}
	}
```

- [ ] **Step 1.4: approve edges now carry the gate** (lines ~192-194) — replace the zero-commands
  assertion (and its comment: the push/PR post must never ship an uncommitted tree):

```go
		// approve pushes the branch and opens the PR — the tree must be fully
		// committed or the PR ships incomplete work (t31).
		assertRuns(t, tc, namedEdge(t, tc, "impl_review", "approved", "approve"), cmdCleanTree)
```

- [ ] **Step 1.5: deliver description carries the KB reminder** — in the existing
  `for _, tc := range []mtt.Type{task, chore}` description-assert loop (lines ~241-245), add:

```go
		if d := namedEdge(t, tc, "approved", "done", "deliver"); !strings.Contains(d.Description, "mtt note add") {
			t.Fatalf("%s deliver description lost the KB-capture reminder: %q", tc.Name, d.Description)
		}
```

- [ ] **Step 1.6: run — must FAIL** (proves the guard bites):
  `go test ./internal/adapter/yaml/ -run TestRepoDogfoodConfig`
  Expected: `FAIL … task edge speccing->spec_review: 1 commands, want 2`.
- [ ] **Step 1.7: no commit yet** — Task 1+2 land as one commit (test+config together, per the
  guard-test-updates-with-config rule in AGENTS "Working under mtt" / SEC2).

### Task 2: Config gates + CHANGELOG (green)

**Files:** Modify: `.mtt/config.yaml`, `CHANGELOG.md`

- [ ] **Step 2.1: add the clean-tree gate** (exact scalar = `cmdCleanTree`): **last** on the 4
  spec/plan submit edges (after the `ls` glob), **first of three** on the 4 impl submit edges, and the
  **only** command on both `impl_review→approved` approve edges — those two edges are already block
  form; just insert a `commands:` key with the single clean-tree scalar, keeping `description`/`post:`
  byte-identical. The 4 impl submits become (order matters):

```yaml
        commands:
          - 'out=$(git status --porcelain -- ":(exclude).mtt") && test -z "$out" || { echo "working tree not clean - commit your code/docs first (.mtt is swept by the move itself)" >&2; false; }'
          - 'git diff --quiet main...HEAD -- cmd internal pkg go.mod go.sum || git diff --name-only main...HEAD -- CHANGELOG.md | grep -q . || { echo "code changed but CHANGELOG.md has no entry - add one under [Unreleased] (pure refactor? bypass: mtt do submit --no-run --who ... --why ...)" >&2; false; }'
          - {run: 'make check', timeout: 10m}
```


- [ ] **Step 2.2: extend both deliver descriptions** (task + chore, currently
  `"after the squash-merge: pull main, then deliver (moves your tree to main and writes done there)"`) to:

```yaml
        description: "after the squash-merge: pull main, then deliver (moves your tree to main and writes done there); before delivering, capture this task's durable lessons: mtt note add"
```

- [ ] **Step 2.3: CHANGELOG.md** — add under `## [Unreleased]` (create the section if absent):

```markdown
### Added
- Dogfood flow gates (t31): every `submit` and both `impl_review → approved` edges now require a clean
  working tree (`.mtt` excluded — the move's own post-commit sweeps it); impl submits additionally
  require a CHANGELOG entry when code changed vs the merge base (audited bypass:
  `mtt do submit --no-run --who … --why …`); `deliver` reminds to capture lessons via `mtt note add`.
```

- [ ] **Step 2.4: run — must PASS:** `go test ./internal/adapter/yaml/ -run TestRepoDogfoodConfig`
- [ ] **Step 2.5:** `make check` → green.
- [ ] **Step 2.6: live smoke of the gate wiring** (t31 is in `implementing`; its submit edge now
  carries clean-tree → CHANGELOG → make check): `touch scratch.tmp && ./bin/mtt submit t31;
  echo exit=$?` → expect `✗` on the clean-tree command (Commands[0]), `exit=3`, task still
  `implementing`. Then `rm scratch.tmp`. This probe MUST run before Step 2.7's commit — the
  uncommitted test/config/CHANGELOG edits guarantee the block. **Hazard: never re-run it on a
  fully-committed tree without the scratch file — with the gates green it would really advance t31
  to `impl_review` mid-implementation.**
- [ ] **Step 2.7: commit:**

```bash
git add internal/adapter/yaml/dogfood_test.go .mtt/config.yaml CHANGELOG.md
git commit -m "t31: mechanize the manual ticks — clean-tree gate on submit+approve, CHANGELOG gate on impl submits, KB reminder on deliver"
```

### Task 3: KB seed — 12 notes

**Files:** Create (via CLI only): `.mtt/knowledge/<slug>.md` × 12

- [ ] **Step 3.1:** create each note with `./bin/mtt note add <slug> --title … --priority … --tag … --file -`
  (body on stdin heredoc `<<'EOF'`). Slugs/priorities/tags/titles + full bodies:

**1. `process-model`** — high, tags `process,flow`, title "Product axis, not process: what mtt tracks here"

```
mtt tracks the PRODUCT axis: a task is a unit of product change; the 15-status task flow is one
task's maturation (spec -> plan -> TDD -> reviews -> delivery), which may span several work sessions.
Sessions/phases (how we work) are process - ephemeral, executed, never queued; modeling them as
tracked items was the s009 category error, caught by asking "what are we actually tracking?".
- Two types, chosen by design-openness: task (design OPEN - spec+plan artifacts, each reviewed) vs
  chore (design ALREADY FIXED elsewhere - impl stage only). A chore whose diff contains undocumented
  design decisions must be declined and recreated as a task (the impl_review description polices this).
- A task leaves the queue only over a flow edge: deliver (verified "done = in main") or cancel --why.
  mtt rm erases the record - it is for mistakes, never closure. "Done directly" (work landed on main,
  task rm-ed - the t43 precedent) is the anti-pattern this note exists to kill: already-decided work
  becomes a chore instead.
- The flow prints its own instructions at every status (descriptions are the runbook); trust
  mtt roadmap / mtt show over any memory of the process.
```

**2. `tag-conventions`** — high, tags `process`, title "Tag conventions: backlog, think, and the thematic vocabulary"

```
- backlog = not in the live queue. Every deferred task carries it; PROMOTING a task = mtt tag rm <id>
  backlog. The live queue = open tasks minus backlog; mtt roadmap is the "what next" view (low-priority
  backlog sinks), mtt list --tag backlog the backlog view.
- think = design-open item (usually "Think:"-titled): brainstorm before implementing; drop the tag once
  the design is decided.
- Thematic tags are a deliberately SMALL vocabulary - currently core, flow, sec, tests, perf, dx, ux,
  kb, adapter, demo, multiagent, release, docs. Pick from the existing set before inventing; discover
  the live set with mtt tags (--all for every task, --json for {tag,count}).
- Caveat: #hashtags in titles/descriptions auto-become tags - never put # in a title unless you mean
  it (a "#2" in a migration title once minted a spurious "2" tag).
```

**3. `adversarial-reviews-pay`** — high, tags `process,tests`, title "Two adversarial subagent reviews per task: the evidence"

```
Every spec+plan pair sent to an adversarial subagent review has produced at least one real,
empirically-verified defect a self-review missed. Keep the loop: spec review, then plan review, each
instructed to REFUTE claims by running probes, not by reading prose. Catches on record:
- fail-open shell gate (! mtt list ... | grep -q . passes when mtt is missing; also t31's
  ! git diff form converting exit 128 into a pass) -> fail-closed shapes only;
- YAML quoting traps (double-quoted gate breaks \., a leading ! becomes a yaml tag) -> single-quote
  command scalars, assert exact strings in the guard test;
- a plan saying "create test file X" when X exists with tests (Write would silently drop them - grep
  before creating; append, no package/import header);
- cobra validates Args BEFORE RunE (a fixed-arity validator rejects bulk forms before classification);
- t31's clean-tree gate would have deadlocked its own task (.mtt dirtied by mtt note add is swept by
  the post-commit AFTER the gate) - the reviewer caught the ordering.
Reviews check FORM empirically; humans sign off at the *_human_review statuses.
```

**4. `working-under-flow-traps`** — high, tags `flow,process`, title "Working under the dogfood flow: traps and recoveries"

```
- Commit code/docs to the task branch BEFORE mtt submit/approve - flow post-hooks auto-commit ONLY
  .mtt. Since t31 this is machine-enforced (clean-tree gate, .mtt excluded); the gate text names the fix.
- Backlog items mtt add-ed to LOCAL MAIN while a task is in flight: before deliver, reset --hard
  origin/main and cherry-pick the post-branch SHAs - deliver greps local main for the squash subject
  and pushes main, so a diverged local main ships stray commits. Prefer adding queue items on the task
  branch (they merge via the PR) or push main immediately.
- Gates inherit the caller's environment: an exported MTT_DIR leaks through a make check gate into the
  testscript suite and reds it deterministically. Prefer cwd discovery + config.local author over
  MTT_DIR/MTT_BY exports.
- Post-action failure (commit/push/PR-open) keeps the move and exits 5; finish by hand - the exit-5
  message prints the recovery commands (t28).
- --no-run skips ALL commands on the edge including context switches: on deliver/cancel it skips
  git switch main, so the terminal write strands on the current branch (t32 caveat). It always demands
  --who and --why (exit 2 otherwise) - a signed, audited bypass, not a convenience.
```

**5. `positioning-vs-beads`** — high, tags `release,docs`, title "Positioning: the wedge, the 2026 landscape, the channel"

```
Wedge (bet #1, verified un-copied as of 2026-07-09): config-driven PER-TYPE flows + BLOCKING
shell-command transition gates in a local zero-footprint CLI. Nobody else combines them: beads has
custom-but-GLOBAL statuses and bd gate = async wait primitives (approval/timer/CI), not command-gated
transitions; Backlog.md has DoD CHECKLISTS the agent ticks (pitch: "everyone has the DoD vocabulary -
mtt makes DoD executable"); Task Master a fixed enum; Claude Code Tasks + TaskCompleted hook = one
global hook, home-dir state, Claude-only, gates only "done".
- The #1 2026 objection is "my harness has a completion hook": answer table - per task type (bugfix !=
  refactor), flow as committed+reviewed REPO DATA not personal settings, harness-portable (any CLI
  agent), gates every lifecycle edge not just done, append-only history with check results.
- Problem validation numbers (cite sparingly, one per pitch): 75% rework issue (claude-code#25305),
  19/19 false "all tests pass" (BSWEN 2026-06), reward-hack exclusion drops SWE-bench Pro 87.1->73.0
  (Cursor 2026-06).
- Channel: an AGENTS.md snippet, not the binary (beads and Backlog.md both grew that way). The channel
  punishes bloat and confusing errors - zero-footprint and crisp errors are RETENTION features.
- Bet #2 (external tracker as store-of-record) stays roadmap material, never headline; adapters follow
  demand (GitHub Issues first, only on real asks).
```

**6. `release-and-launch`** — medium, tags `release`, ref `task:t60`, title "Release mechanics + launch plan essentials"

```
- Version = git-describe stamp (t30): no version constant to bump per change; semver decided at tag
  time; RELEASING.md is the runbook; cadence is batched and on-demand, never per-PR.
- Launch trigger (decided 2026-07-10): a tagged release + the dogfood proof - do NOT wait for an
  external tracker adapter. Dogfood IS the launch asset: "mtt's own development passes through its own
  gates", demoed by a real history excerpt with checks/who/why.
- Launch assets in priority order: (1) the copy-pasteable AGENTS.md adoption snippet in README;
  (2) a "how mtt gates its own development" write-up with one honest what-it-does-not-do section
  (cooperative discipline, not a jail; --no-run exists and is signed); (3) README positioning surface
  (vs-harness-hooks section, refreshed scan, gate-naming tagline); (4) Show-HN / r/ClaudeAI leading
  with enforcement ("your agent can't say done until the gate is green"), never "task tracker".
- Wave-2 signals: build the GitHub Issues adapter on recurring real "can it write to my tracker?"
  asks; if "how is this different from hooks?" dominates, iterate the POSITIONING, not features.
- The channel is one-shot: known warts (invisible gate cause, broken documented pattern) burn the
  launch - polish the checklist, timebox it, cut scope rather than slip.
```

**7. `testscript-e2e-conventions`** — medium, tags `tests`, title "testscript/e2e conventions that bite"

```
- Assertions must anchor, not substring-match: 't1  task  \[tbd\]', (?m)^id$ for line-oriented output;
  a loose substring matches vacuously. stdout regexp is whole-output.
- No shell pipes: model a|b as exec a -> cp stdout f -> stdin f -> exec b - (stdin resets per command).
- Wall-clock timestamps tie at second resolution: e2e asserts ordering RELATIONSHIPS ((?s)t1.*t2),
  never absolute positions; exact order is a unit test with a fixed clock.
- A gated e2e config ships as txtar -- gated.yaml -- cp-ed over .mtt/config.yaml, and must be a VALID
  flow: mtt add runs Config.Validate, a 2-status initial->terminal flow dies at the first add - use
  initial->active->terminal even for a minimal demo.
- git in testscript: guard with [exec:git], git init -b main + user.name/email + one --allow-empty
  commit (git switch from an unborn HEAD exits 128); assert an unborn branch via git symbolic-ref
  --short HEAD, not git branch --list.
- Gate-output needles must be OUTPUT-only: the runner echoes commands on progress lines, so a needle
  that substrings the command text proves nothing - use computed output (echo $((13*13)) -> 169).
- A blocked-move e2e must pin the CAUSE on stderr: with require active, ! exec alone cannot tell an
  attribution exit-2 from a gate exit-3.
- txtar support files unpack into $WORK; after cd proj reference them as $WORK/file.
```

**8. `go-cli-conventions`** — medium, tags `dx`, title "Go/cobra conventions this codebase enforces"

```
- cobra validates Args BEFORE RunE, on flag-stripped positionals: a context-sensitive command needs a
  context-sensitive PositionalArgs closure (it receives cmd with flags parsed), not a fixed arity.
- CLI output goes through fmt.Fprint(cmd.OutOrStdout(), ...) - cmd.Print* routes to stderr when no
  writer is set, breaking pipes and stdout asserts.
- Root sets SilenceErrors, so a unit test asserts the RETURNED error, not a SetErr buffer (the e2e
  harness differs: cli.Execute prints to real stderr).
- golangci unused fails on declared-but-unused package symbols: declare a symbol in the task that
  FIRST uses it; transient IDE unused diagnostics during multi-edit wiring are noise - make check is
  the gate.
- Exit-code taxonomy lives in Execute() int via errors.Is on core sentinels: wrap with %w everywhere
  (a %v silently degrades exit 4 to 1); a bulk best-effort aggregate must be a PLAIN fmt.Errorf -
  %w-wrapping one per-item error mis-maps the whole bulk.
- Zero-match --json emits [] not null: build with make([]T, 0, ...).
- Verb sugar rides root.RunE fallback with ArbitraryArgs (a real subcommand always wins the clash);
  route new forms to the OLD path (resolve edge-name -> target, call the existing runTransition) so
  gates/attribution/exit codes are inherited, not re-implemented.
```

**9. `architecture-heuristics`** — medium, tags `core`, title "Recurring architecture decision tests"

```
- Port-vs-field (GAP #1): can the reference adapter embed it in the aggregate? Yes -> Task field +
  TaskStore.Update, no new port (depends_on, tags, priority, history). No (non-embeddable, e.g. the
  personal current pointer) -> a capability port. Delete cannot be embedded -> base-port method.
- Value objects: closed vocabulary -> type + consts + Valid(), cast in toDomain, validated at the CLI
  boundary, NO smart constructor (StatusKind/Priority/CurrentAction idiom). Open TRANSFORMING
  vocabulary (tags) -> plain []string + pure functions. Named identities -> reject empty, never
  transform.
- Domain-vs-policy for a per-edge property: authored on the specific edge -> domain VO (per-command
  timeout); a runner-wide default -> adapter Settings (global command_timeout).
- Derived graphs (children index, dep graph, roadmap ordering) are computed in core from List, never
  stored, never in pkg/mtt; do not force a shared traversal until a third consumer demands it.
- A pure read needs no core usecase (show/list compose store + pure functions); only mutations get
  usecase structs, clocked via injected now.
- DTO field drops are a silent-bug class: a domain field the DTO does not map dies at Load with green
  tests - test new fields THROUGH Load/toDomain, and audit optional DTO fields when a domain knob
  "does nothing".
- Measured scale posture (2026-07-10, N=5000): list/tree/dep linear (~120ms), gated status O(1) (3ms),
  roadmap ~quadratic (1s; heap fix when real volume demands); the gate path never depends on N.
- Trust model: config is code (Makefile-class); placeholder expansion exposes exactly {ID,Type,From,To}
  via a template struct - free text structurally cannot reach the shell; the binary is zero-network.
```

**10. `flow-authoring-lessons`** — medium, tags `flow`, title "Authoring flow configs: the hard-won rules"

```
- Descriptions are load-bearing (the flow IS the runbook printed at each status): guard-test them like
  commands - Config.Validate runs on add/types, NEVER on Load or the move path, so the repo guard test
  asserting EXACT strings is the sole protection against silent YAML mangling.
- Single-quote command scalars: a double-quoted scalar eats backslashes (\.mtt breaks), a bare leading
  ! parses as a yaml tag and vanishes.
- Gates must be fail-closed: out=$(cmd) && test -z "$out" (an operational failure lands in the error
  branch). NEVER negate a command (! cmd) to express "no diff/no output" - negation converts exit 128 /
  missing-binary into a PASS (caught twice: the s009 self-ref gate, the t31 changelog gate draft).
- Commands run PRE-write: an edge that must land its state write elsewhere switches the tree first
  (git switch main on deliver/cancel) and re-guards with test -f after the switch.
- Gates see the caller's working tree and env; order gates cheap-first.
- Invariant-rejection fixtures must isolate exactly ONE violation (a self-loop edge is a clean
  isolator); verify the fixture fails on the intended invariant, not a neighboring one.
- e2e proves the MECHANISM with generic POSIX commands (touch/false), not git; precise semantics
  (reverse order, best-effort) are unit tests against the fake runner.
```

**11. `git-github-traps`** — medium, tags `dx`, title "git/GitHub integration traps (verified live)"

```
- GitHub squash-merge takes the subject FROM THE COMMIT on single-commit PRs unless the repo sets
  squash_merge_commit_title=PR_TITLE (this repo flipped it; verified via gh api). Any convention keyed
  to "the PR title reaches the squash subject" depends on that setting.
- Branch protection on main would break the deliver/cancel post-push (exit 5) - the delivery tail
  assumes direct push (t33 tracks team semantics).
- git switch from an unborn HEAD exits 128; a fresh e2e repo needs one commit before switching.
- go get pkg@latest can raise the go directive floor of go.mod (a compat break for downstream Go
  toolchains): pin a floor-compatible version explicitly, then go mod tidy, and re-check go.mod still
  says the intended floor.
- gh pr create is made idempotent by guarding on gh pr list --head ... --state open (the approve post
  pattern); body from docs/superpowers/pr/<id>.md when present.
```

**12. `dogfood-history`** — low, tags `docs`, title "Where the history lives (orientation breadcrumb)"

```
mtt bootstrapped itself over sessions s001-s009 (2026-07-03..11): contract -> YAML store -> hierarchy
-> dependencies -> gated flow -> attribution/current/structured commands/rollback -> tags, priorities,
batch -> release prep -> s009 self-host (flow v2: verified delivery tail, two types, auto-commit/push/
PR posts). Since s009 every product change is an mtt task on a task/<id> branch.
- t31 (2026-07-23) retired the pre-mtt apparatus: TASKS.md (the bootstrap plan), sessions/*.md (the
  narrative archive), NEXT_SESSION.md (the handoff primer), and delivered-task artifacts under
  docs/superpowers/. Git history keeps all of them; nothing was lost, only de-canonized.
- Where things live now: queue = mtt roadmap; knowledge = mtt notes (mtt prime at session start);
  architecture = DESIGN.md; rules = AGENTS.md; per-task artifacts = docs/superpowers/{specs,plans,pr}/
  <id>-*.md for OPEN tasks only (delivered ones are deleted - git has them).
```

- [ ] **Step 3.2: verify:** `./bin/mtt note list` shows 12; `./bin/mtt prime` prints exactly the five
  high notes; `./bin/mtt note show tag-conventions` renders.
- [ ] **Step 3.3: commit:**

```bash
git add .mtt/knowledge && git commit -m "t31: KB seed — 12 curated notes (carry-over lessons + positioning distilled)"
```

### Task 4: Backlog verification + t63 extension

**Files:** none directly (`.mtt` via CLI; committed here)

- [ ] **Step 4.1:** cross-check the un-filed items from
  `docs/superpowers/notes/2026-07-10-debt-security-tests-triage.md` (disposition row "backlog /
  think-items") against `./bin/mtt list --json | jq -r '.[].title'`: T2 (invalid-config e2e), T3
  (`rm --dry-run --json`), T4 (`writeDepCycles` cycle unit), T5 (stale-current sugar), T8 (full-stack
  exit-code asserts), concurrent-mint + `atomicWrite` failure modes, roadmap heap fix (quadratic),
  SEC4 (artifact signing). t58/t61/t10 cover integrity/arch/concurrency clusters — anything not
  covered gets ONE batch task:

```bash
./bin/mtt add "test-suite gap batch (2026-07-10 audit): invalid-config e2e, rm --dry-run --json, writeDepCycles cycle, stale-current sugar, full-stack exit-code asserts, concurrent mint + atomicWrite failure modes" --tag backlog,tests --priority low
./bin/mtt add "release artifact signing / gh attestation (SHA256SUMS is integrity, not authenticity)" --tag backlog,sec,release --priority low
```

  The roadmap-heap fix is NOT added — it is already filed as **t13** ("performance: roadmap heap …");
  record "heap → covered by t13" in the verification notes.

  (Skip either of the two whose content verifiably already lives in an open task; also skim
  `2026-07-09-s009-readiness-and-architecture-audit.md` + `2026-07-09-flow-granularity-for-dogfood.md`
  + the two `.ru` notes for action items not shipped and not filed — same rule: file or confirm covered.)
- [ ] **Step 4.2: extend t63's description** — full replacement text (current text + the four files):

```bash
./bin/mtt edit t63 --description "Follow-up to t31 (decided in the 2026-07-23 brainstorm, deliberately split out): DESIGN.md accumulated per-task Shipped narrative blocks that are history, not current architecture. Distill them into KB notes (the t31 note set establishes the pattern), keep DESIGN.md (and the RU mirror in sync) as the source of truth for the CURRENT architecture only. Guard: this is curation, not wholesale copy - avoid creating a second desyncing copy (the t53 parallel-occurrences trap). Overlaps t42 (user-docs audit) - coordinate so the two sweeps do not fight. Also owns the four session spec files t31 deferred because live DESIGN Shipped blocks cite them by path (delete or rewrite the citations together with the unload): docs/superpowers/specs/2026-07-09-session-009-dogfood-design.md, 2026-07-11-flow-v2-mechanized-delivery-design.md, 2026-07-09-session-008.7-tags-design.md, 2026-07-09-session-008.9-batch-design.md."
```

- [ ] **Step 4.3: commit:** `git add .mtt && git commit -m "t31: queue — file audit-gap backlog tasks; t63 owns the four deferred session specs"`

### Task 5: Purge

**Files:** Delete (git rm): see exact lists

- [ ] **Step 5.1: apparatus files:**

```bash
git rm NEXT_SESSION.md TASKS.md
git rm -r sessions
```

- [ ] **Step 5.2: delivered-task artifacts** — delete ALL of `docs/superpowers/pr/` (14 files: c9 c10
  t4 t14 t16 t23 t27 t28 t29 t30 t44 t45 t47 t50), ALL id-keyed specs+plans except `t31-*` (specs: t1
  t4 t5 t14 t16 t19 t21 t23 t27 t28 t29 t30 t44 t45 t47 t50 t51 t57; plans: same set), ALL 20
  session-named plans (`2026-07-*`), and 16 of 20 session-named specs — keep exactly these four
  (t63-deferred): `2026-07-09-session-008.7-tags-design.md`, `2026-07-09-session-008.9-batch-design.md`,
  `2026-07-09-session-009-dogfood-design.md`, `2026-07-11-flow-v2-mechanized-delivery-design.md`.

```bash
git rm docs/superpowers/pr/*.md
cd docs/superpowers/specs && git rm t1-*.md t4-*.md t5-*.md t14-*.md t16-*.md t19-*.md t21-*.md t23-*.md t27-*.md t28-*.md t29-*.md t30-*.md t44-*.md t45-*.md t47-*.md t50-*.md t51-*.md t57-*.md \
  2026-07-03-*.md 2026-07-04-*.md 2026-07-05-*.md 2026-07-06-*.md 2026-07-07-*.md \
  2026-07-09-chore-008.95-release-prep-design.md 2026-07-09-flow-guidance-on-entry-design.md \
  2026-07-10-named-transitions-design.md && cd ../../..
git rm docs/superpowers/plans/2026-07-*.md
cd docs/superpowers/plans && git rm t1-*.md t4-*.md t5-*.md t14-*.md t16-*.md t19-*.md t21-*.md t23-*.md t27-*.md t28-*.md t29-*.md t30-*.md t44-*.md t45-*.md t47-*.md t50-*.md t51-*.md t57-*.md && cd ../../..
```

  Safety re-check before running: `./bin/mtt list --kind initial --kind active --ids` must contain
  none of the ids being deleted (t31 stays, its files are not in the lists above).
- [ ] **Step 5.3: notes** (Task 3+4 distilled/verified them): `git rm docs/superpowers/notes/*.md`
- [ ] **Step 5.4:** `make check` (nothing compiles against docs — expected green; catches accidental
  test-data deletion). Commit:
  `git commit -m "t31: purge the pre-mtt apparatus — NEXT_SESSION, TASKS, sessions/, delivered-task artifacts (git history keeps them)"`

### Task 6: Pointer rewiring (EN + RU)

**Files:** Modify: `DESIGN.md`, `DESIGN.ru.md`, `CLI_REFERENCE.md`, `CLI_REFERENCE.ru.md`, `README.md`,
`README.ru.md`

Transforms: live deferred item → real mtt id; historical citation → drop the pointer clause (plain-text
history mentions stay; **no markdown links to deleted files**). RU mirror gets the equivalent edit at
the listed line (fragments verified by grep).

- [ ] **Step 6.1 DESIGN.md** (RU line in parens):
  - :484 (ru :488) `See sessions/008 and TASKS.md → e4_t10.` → delete the sentence.
  - :509 (ru :513) `See sessions/007 and TASKS.md → e4_t9.` → delete the sentence.
  - :532 (ru :537) `See TASKS.md → Later.` → `Tracked in mtt` + the matching open-task id: run
    `./bin/mtt show t11` — if its title matches this WIP-commit/completeness item use `(t11)`, else
    cite `mtt roadmap` generically.
  - :556 (ru :561) `See TASKS.md → e4_t8a.` → same procedure with `./bin/mtt show t18` (current
    follow-ups) / `t13`; fallback `mtt roadmap`.
  - :618 (ru :623) `See TASKS.md → Later ("current" vs roles).` → `Tracked as t18 (current-vs-roles
    follow-ups).` (verify t18's title first, same fallback).
  - :627 (ru :632) `See TASKS.md → Later.` → `Tracked as t3 (actor profiles).`
  - :746 (ru :753) `(see TASKS.md → Later)` → `(tracked in t10 — the multi-agent concurrency cluster)`.
  - :756 (ru :764) `See TASKS.md → Later.` → `Tracked as t36 (cancelled-blocker semantics revisit).`
  - :917 (ru :929) `See sessions/README.md → "Roadmap regrouped".` → delete the sentence (the preceding
    sentence already says dogfood happened).
  - :922 (ru :934) `that stays in \`sessions/*.md\` + git` → `that stayed in session notes, since
    retired to git history (t31)`.
  - :928 (ru :940) gate inventory: after `id-keyed artifact presence (\`ls docs/superpowers/specs/<id>-*.md\`) on spec/plan submits,` insert
    `a clean-working-tree check (\`.mtt\` excluded) on every submit and on approve (t31), a CHANGELOG-entry check on impl submits when code changed (t31),` before `\`make check\``.
- [ ] **Step 6.2 CLI_REFERENCE.md:** :317 (ru :316) `tracked in TASKS.md → Later.` → `tracked in t10
  (the multi-agent concurrency cluster).`; :769-770 (ru :761) delete the parenthetical
  `*(TASKS.md still mentions … fold it into \`done\`/\`cancel\`.)*`.
- [ ] **Step 6.3 README.md:134 / README.ru.md:134:** replace the TASKS.md bullet with the dogfood line:
  - EN: `- The backlog is dogfooded: this repo tracks its own tasks in mtt — run \`mtt roadmap\` in a checkout`
  - RU: `- Бэклог — догфуд: репозиторий ведёт свои задачи в mtt — запустите \`mtt roadmap\` в чекауте`
- [ ] **Step 6.4 sweeps** (definition of done for this task):

```bash
git grep -n 'make check' DESIGN.md DESIGN.ru.md        # update any missed flow-fact restatement
git grep -n 'TASKS\.md\|NEXT_SESSION\|sessions/' -- ':(exclude).mtt' ':(exclude)CHANGELOG.md' ':(exclude)docs/superpowers'
git grep -n 'docs/superpowers/' -- ':(exclude).mtt' ':(exclude)CHANGELOG.md' ':(exclude)docs/superpowers' ':(exclude).claude'
```

  Remaining hits must be: AGENTS/CLAUDE (rewritten next task), config.yaml artifact globs (live), the
  four kept spec citations in DESIGN(+ru), FLOW_GUIDE mentions of `docs/superpowers` paths if any refer
  to the artifact convention generically (fine — the dirs still exist). Fix everything else by the two
  transforms. Then `make check` + commit:
  `git commit -am "t31: rewire frozen-apparatus pointers to live mtt ids; update the gate-inventory flow-fact (EN+RU)"`

### Task 7: Rules — AGENTS.md + CLAUDE.md

**Files:** Modify: `AGENTS.md`, `CLAUDE.md`

- [ ] **Step 7.1 AGENTS.md:**
  - "Documentation language" list → `**Agent-facing docs are English only:** \`AGENTS.md\`, the
    \`CLAUDE.md\` files.` (drop TASKS/NEXT_SESSION).
  - Replace the whole "Sessions → tasks" section body with:

```markdown
The unit of work is an **mtt task** on a flow-created `task/<id>` branch; the method steps (brainstorm →
spec → plan → TDD → reviews) are printed by the flow itself at each status. The pre-self-host session
apparatus (`sessions/`, `TASKS.md`, `NEXT_SESSION.md`) is retired (t31): narrative history lives in git;
orientation lives in the KB (`mtt note show dogfood-history`).
```

  - In the "The backlog is in mtt" bullet (AGENTS.md:150), rewrite `sessions/phases (how *we* work)
    stay in \`sessions/*.md\` — they are not mtt tasks` → `sessions/phases (how *we* work) are
    process — executed, never queued (see \`mtt note show process-model\`)`.
  - "Working under mtt": drop the `TASKS.md is frozen` clause from the intro sentence; replace the tag
    block's convention text (keep the first backlog/promote sentence) with
    `Tag semantics and the thematic vocabulary live in the KB: \`mtt note show tag-conventions\`.`;
    delete the `(This tag-convention note is interim — it migrates into mtt later.)` parenthetical
    (it just did); add two bullets:

```markdown
- **Closure is a flow edge.** A task leaves the queue only via `deliver` (after the squash-merge) or
  `cancel --why`. `mtt rm` is NOT closure — it erases the record (mistakes/duplicates only). "Done
  directly" (landing work on main and rm-ing the task) is forbidden: work whose design is already
  decided becomes a `chore` and rides the chore flow.
- **Knowledge goes to the KB.** Durable lessons and decisions → `mtt note add` (session start reads
  `mtt prime`); markdown files are neither a task-state nor a knowledge channel, and the only
  "what's next" source is `mtt roadmap`. No parallel state docs.
```

  - "Tests" section first bullet → `Unit tests: \`core\` (usecase) / \`adapter/yaml\` — table-driven
    where the shape fits.` (the audit's honesty fix).
- [ ] **Step 7.2 CLAUDE.md:**
  - Line 3-4: `Full rules — in [AGENTS.md](AGENTS.md), architecture — in [DESIGN.md](DESIGN.md), the
    live queue — \`mtt roadmap\`.` (drop the TASKS.md parenthetical).
  - "Read at the start of a session:" → `AGENTS.md → DESIGN.md → \`mtt roadmap\` + \`mtt prime\`.`
  - Docs-language line → `Agent-facing docs (this file, AGENTS.md) are English.` (rest unchanged).
  - "Skills / guards" section: keep the first sentence, replace the NEXT_SESSION pointer with the
    inlined activation:

```markdown
Plugins load at session start; on first open confirm the marketplace trust prompt. If the skills
don't appear, run once: `/plugin marketplace add obra/superpowers-marketplace` then
`/plugin install superpowers@superpowers-marketplace` (alternative:
`superpowers@claude-plugins-official`), and verify the TDD/brainstorming/debugging skills are active.
```

- [ ] **Step 7.3:** `make check` + commit:
  `git commit -am "t31: mtt-first rules — closure via flow edges only, KB as the knowledge channel; superpowers activation inlined into CLAUDE.md"`

### Task 8: Acceptance verification + submit

- [ ] **Step 8.1:** full sweep once more (same three greps as Step 6.4) — zero unexplained hits; plus
  `git grep -rn 'TASKS\|NEXT_SESSION' docs/architecture demo` (model.go/demo were never grepped) — fix
  by the same transforms if anything surfaces.
- [ ] **Step 8.2:** `./bin/mtt prime` → the 5 high notes; `./bin/mtt note list` → 12;
  `./bin/mtt show t63` shows the extended description; `./bin/mtt roadmap` still renders.
- [ ] **Step 8.3:** `make check` → green.
- [ ] **Step 8.4: live negative probe** (acceptance #4; t31 in `implementing`, tree otherwise fully
  committed — this run isolates the scratch file as the blocker): `touch scratch.tmp &&
  ./bin/mtt submit t31; echo exit=$?` → `✗ … working tree not clean` on Commands[0], `exit=3`,
  status unchanged; `rm scratch.tmp`.
- [ ] **Step 8.5: the real submit:** `./bin/mtt submit t31` — clean-tree passes, the CHANGELOG gate
  passes (the Task 2 entry), `make check` (10m timeout) runs green; task → `impl_review`.
- [ ] **Step 8.6: approve-edge probe** (the second half of acceptance #4), at `impl_review` BEFORE the
  adversarial code review concludes: `touch scratch.tmp && ./bin/mtt approve t31; echo exit=$?` →
  `✗` clean-tree, `exit=3`, status unchanged; `rm scratch.tmp`. The real `mtt approve` happens only
  after the impl_review adversarial code review passes (per the flow).

### After the flow reaches `done` (outside the PR; on main)

- [ ] `./bin/mtt cancel t53 --why "resolved by t31: lessons distilled into the KB, artifacts archived to git history; the DESIGN unload remainder is t63"`
- [ ] `./bin/mtt cancel t54 --why "mechanized as the CHANGELOG gate on impl submits in t31"`
- [ ] `./bin/mtt roadmap` — sanity: t63 unblocked, queue coherent.

## Plan self-review notes

- Spec coverage: D1→Task 3; D2→Tasks 1-2 (+8.4 live proof); D3→Tasks 5-6 (+4.1 note-verification,
  +8.1 sweep); D4→Task 7; acceptance 1-5→Tasks 1,3,6,8; acceptance 6→the post-done block; t63
  extension→Task 4. Non-goals respected: no production Go code (only dogfood_test.go).
- The Task 2.6 probe intentionally runs pre-commit (dirty tree guaranteed); the Task 8.4 probe proves
  the same on an otherwise-clean tree with only the scratch file.
- Deletion lists were cross-checked against the open-task set during spec review (all id-keyed
  artifacts belong to terminal tasks; re-checked mechanically in Step 5.2).
