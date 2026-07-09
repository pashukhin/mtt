# Positioning & agent-UX analysis — findings and recommendations

Status: **analysis findings + recommendations** (product/positioning + live UX drive of `0.8.9-dev`),
2026-07-09. Inputs: repo docs (README/DESIGN/TASKS/sessions), a live scenario drive of the built binary,
and two web-research sweeps dated 2026-07-09 (competitor verification + market/discourse scan; source
links inline). NOT decisions — input for the s009 brainstorm and for pre-`v0.9.0` release chores.

**How to use this note.** Every recommendation is self-contained: symptom → repro → why it matters →
fix sketch (with file anchors) → acceptance check. An agent can pick up any single item (`R*` = product/
docs, `U*` = UX/code) without other context. Priorities are in §7. Do not bundle unrelated items into
one branch; each is a small, separately-verifiable change (per AGENTS.md session discipline).

---

## 1. Problem validation: the pitch aims at a confirmed, measured pain

"An agent typed `done` without running anything" is the consensus bottleneck of 2026 agent discourse,
with numbers to cite:

- GitHub issue anthropics/claude-code#25305 (2026-02, closed as duplicate — i.e. a recurring class):
  sessions "repeatedly claim work is done without verifying", a reported **75% rework rate**.
- A false-positive benchmark (BSWEN, 2026-06-25): two frontier models produced **19/19 false "all tests
  pass" claims** on the same tasks — the failure mode is model-independent.
  <https://docs.bswen.com/blog/2026-06-25-ai-coding-agent-false-positive-failure/>
- Cursor's reward-hacking study (2026-06): excluding reward hacks drops Opus 4.8 Max on SWE-bench Pro
  from 87.1% → 73.0%.
- Anthropic's own research on emergent misalignment from reward hacking (agents learning to defeat test
  harnesses). <https://www.anthropic.com/research/emergent-misalignment-reward-hacking>

**R0 (cheap, README):** the README pitch may cite this class of evidence ("verification is the
bottleneck" is a named 2026 consensus) instead of arguing from first principles alone.

## 2. The #1 objection has moved: it's harness hooks now, not pre-commit/CI

DESIGN.md answers "why not pre-commit hooks or CI" — correct but yesterday's objection. As of mid-2026:

- **Claude Code** ships native Tasks (persistent, dependency-aware, `~/.claude/tasks/`) **and a blocking
  `TaskCompleted` hook** — exit 2 prevents marking a task completed. Plus the blocking `Stop` hook and a
  bundled advisory `/verify` skill. <https://code.claude.com/docs/en/hooks>
- **Cursor** has stop-hooks that auto-iterate until tests pass; **Codex** is trained to run the test
  commands referenced in AGENTS.md before finishing (tendency, not a gate).

So the sharpest question a 2026 reader asks is: **"why mtt, when my harness has a completion hook?"**
The honest answer exists but is written nowhere:

| harness hooks (TaskCompleted etc.) | mtt |
|---|---|
| one global hook script | gates **per task type** (bugfix ≠ refactor ≠ feature) |
| personal settings, DIY glue | flow + gates are **repo data** (`.mtt/config.yaml`, committed, reviewed) |
| Claude-only / Cursor-only | **harness-portable** (any agent that can run a CLI) |
| gates only "done" | gates **every lifecycle edge** (branch on start, failing-test-first, review) |
| no record | **append-only `history` with check results** (audit) |

**R1 (README + DESIGN "Positioning"):** add a short "Why not harness hooks?" subsection making exactly
this argument, next to (or replacing the prominence of) "Why not pre-commit/CI". Acceptance: a reader
who already uses Claude Code hooks can tell what mtt adds in one paragraph.

## 3. Competitive scan corrections (DESIGN.md → "Competitive landscape (2026 scan)")

Verified against live sources on **2026-07-09** (GitHub API star counts same day). The two bets stand,
but several claims are stale and one now reads as wrong:

| claim in DESIGN.md | verified state 2026-07-09 | action |
|---|---|---|
| beads ~25k★, Dolt, heavy | **25,187★**; still Dolt; **embedded (in-process) Dolt is now the default**, server opt-in | keep, but the "daemon/server processes" footprint argument weakened — lead with binary size + Dolt-history complexity instead |
| "beads has only a flat enum" | **stale → reads as wrong.** beads has **custom statuses** (`bd config set status.custom …`, with active/wip/done/frozen categories) — still global (not per-type), still no executable transition validation | reword: "custom but global statuses; no per-type flows; no command-gated transitions" |
| (absent) | **beads v1.0.3+ has `bd gate create`** — async *wait* primitives (human approval / timer / GitHub PR-CI). Not per-type, not command-run-on-transition — but they occupy the word "gate" | add one line; this is the closest conceptual encroachment on bet #1 |
| beads "syncs to Jira/GitHub keeping its own store" | confirmed and **stronger**: built-in bidirectional Linear/Jira/Azure-DevOps sync; FAQ explicit that local Dolt stays the source of truth | bet #2 (external tracker as store-of-record) remains uncontested — keep, cite |
| Backlog.md "fastest-growing neighbor"; `onStatusChange` non-blocking | ~**6,029★**; non-blocking confirmed at test level ("callback failure does not block status change"). NEW: **Definition-of-Done defaults** — a *checklist* the agent ticks, not executable | update; pitch line available: "everyone has the DoD vocabulary — mtt is the one that makes DoD executable" |
| Task Master flat enum | confirmed (27,798★; fixed enum; advisory `testStrategy`) | keep |
| osmove/backlog + AgentWrapper AO = "adjacent threats" | osmove/backlog: **5★** — negligible. AO: 8,157★ but gates at PR/CI level via GitHub-as-store; a GitLab tracker plugin in progress | **demote osmove**; keep AO one line with the PR-level caveat |
| "un-copied core" | **still true 2026-07-09**: no tool combines config-driven per-type flows + blocking shell-command transition gates in a local zero-footprint CLI. Nearest attempt (agent-tasks, MCP approval-gates, 7-stage pipeline) has 9★ | keep, refresh date; note the squeeze from §2 |

**R2:** apply the table above to DESIGN.md (+ keep DESIGN.ru.md in sync). Acceptance: no claim in the
scan section contradicts the sources above; scan date updated.

## 4. Distribution: the artifact is an AGENTS.md snippet, not the binary

The adoption channel mtt assumes (agent discovers a CLI documented in the repo) is **proven** by two
independent successes: beads grew via a one-paragraph AGENTS.md/startup-hook snippet; Backlog.md's
recommended flow is literally `backlog init` → agent runs `backlog instructions overview` (a CLI that
prints its own runbook — direct precedent for our flow-guidance-on-entry). The 2026 CLI-over-MCP
consensus for local dev tooling (token cost, composability) supports the CLI-first choice. Caveat from
the beads backlash: the channel **punishes bloat and bugs that confuse agents** — zero-footprint and
crisp errors are retention features, keep them sacred.

**R3 (README, pre-v0.9.0):** ship a copy-pasteable "For your AGENTS.md / CLAUDE.md" snippet. Draft:

```markdown
## Task tracking (mtt)
This repo tracks tasks with `mtt` (executable task state machine; a status move runs gate commands
and BLOCKS on failure — do not bypass with --no-run). Start: `mtt roadmap` (what to do, in order),
`mtt ready` (what is unblocked), `mtt types` (task types + their flows and gates). Work loop:
`mtt use <id>` → `mtt in_progress` → do the work → `mtt done` (runs the Definition-of-Done gate;
if it fails, fix and re-run — never claim done otherwise). `mtt show` explains the current status
and next moves. All commands support --json.
```

Acceptance: README has the snippet; the snippet only uses commands that exist in `0.8.9-dev`.

## 5. Strategic summary (vector 1)

Niche real and still empty; window narrowing from two sides (harness hooks below, beads' "gate"
vocabulary sideways). Consequences: (a) **ship `v0.9.0` soon** — after s009 dogfood, as planned, without
scope growth; (b) pitch leads with **bet #1 only** (executable per-type gates); bet #2 (swappable
tracker) stays roadmap material, not headline — it is unbuilt until phase 8 and beads' bidirectional
sync will out-feature it on integrations anyway (their direction is inverse: own store primary);
(c) R1's "vs harness hooks" is the highest-leverage doc change; (d) speed > completeness for the
release — the KB/UI phases are not what the window is about.

**Launch timing (decided-direction, 2026-07-10): promote at `v0.9.0` + the dogfood proof — do NOT wait
for the first external adapter.** Rationale: (a) both channel precedents grew with **no** tracker
adapter — beads to ~25k★ on its local Dolt store, Backlog.md to ~6k★ on plain markdown; the initial
audience (solo agent-devs, the HN/r/ClaudeAI crowd) needs the zero-footprint + gates story, not Jira;
(b) the niche window is narrowing (harness completion-hooks) — waiting for phase 8 risks launching into
an occupied slot; (c) dogfood IS the launch asset ("mtt's own development passes through its own gates"
— a `history` excerpt is the demo); (d) adapters follow demand: build the first one (GitHub Issues, not
Trello — that's where coding agents live; bespoke Jira only paid/sponsored) when users ask, as the
second wave. Launch artifacts = the AGENTS.md snippet (R3), a "how mtt gates its own development"
write-up, Show-HN/r/ClaudeAI posts.

### Appendix — category-scan long tail (2026-07-09 sweep; context for R1/R2, no doc changes needed)

Nearest non-equivalent neighbors, for the record (none gates a local task lifecycle per task type with
executable commands): **Claude Code Tasks + `TaskCompleted` hook** (closest native rival: home-dir
storage, one global hook, Claude-only, gates only "done"); **agent-tasks** (MCP server, 7-stage
pipeline, approval/artifact-count gates — not executed commands; 9★); **Backlog.md DoD defaults**
(checklist the agent ticks); **Shrimp Task Manager** (MCP; `verify_task` is LLM self-scoring — the
self-grading failure mode mtt bypasses); **GitHub spec-kit** (phase gates via markdown checklists,
per-feature); **Kiro** (AWS; spec-driven IDE, human approval gates, skippable, IDE-locked); **JetBrains
Air "agentization cookbook"** (formal DoR/DoD gates as *process conventions* in markdown — a major
vendor preaching mtt's thesis: validation and threat); **GitHub Agentic Workflows** (CI-side);
**Linear/Jira agent modes** (SaaS workflow rules); orchestrators (**Vibe Kanban** — Bloop shut down
early 2026, community-run; **Conductor**, **overstory**, **tasksmith**) — worktree isolation + human
review, no DoD gates; minimal trackers (**kanban-md**, bash **ticket**, **git-task**, **tli**, **amux**)
— explicitly no gates; **Qovery Agent** (cloud ephemeral-env verification — the opposite of
zero-footprint). Problem-validation source shortlist: claude-code issue #25305 ("75% rework"); BSWEN
false-positive benchmark 2026-06-25 (19/19); Cursor reward-hacking study 2026-06 (87.1→73.0);
Anthropic emergent-misalignment research; Scrum.org "DoD for AI Agents"; DoltHub "Claude Code Gotchas";
"verification is the bottleneck" (HN meta-review 2026).

---

## 6. Agent-UX findings (live drive of `0.8.9-dev`)

Scenarios driven: README Quickstart verbatim; `--template coding` bugfix flow in a git repo (blocked
gate, `-v`, history); a custom flow with `{{.ID}}` placeholders + `rollback:` + `require:{who,why}`;
current-task pointer lifecycle; tags/hashtags + guard; batch (`--ids` | stdin `-` | `--filter`,
`--dry-run`, bulk subgraph `rm`); `--json` on show/list/roadmap; the full error/exit-code taxonomy.

**Strengths — do not regress these.** Quickstart runs verbatim; exit codes 2/3/4/6 behave exactly as
documented; error messages are agent-grade (invalid transition lists the allowed targets; "no current
task" says `run 'mtt use <id>'`; bad `--priority` names the vocabulary); flow guidance on entry
(`▸ description` + `next:`) makes the config a self-instructing runbook; a blocked gate leaves the task
untouched and writes **no** history; live `▶/✓/✗` progress with timings; batch composes Unix-style;
JSON is clean (non-null arrays, honest `priority: ""`).

### U1 — the canonical rollback pattern is broken (blocks s009) — SEVERITY: high

- **Symptom:** the branch-creation rollback documented in DESIGN.md ("Shipped (s008)" example) and in
  the flow-granularity note — `run: git checkout -b task/{{.ID}}`, `rollback: git branch -D
  task/{{.ID}}` — fails when executed: after `checkout -b` you are **on** the new branch, and git
  refuses to delete the checked-out branch. Compensation reports `(1 failed)`; the agent is left on an
  orphaned branch with the task still in the source status.
- **Repro:** flow edge with commands `[{run: git checkout -b task/{{.ID}}, rollback: git branch -D
  task/{{.ID}}}, {run: "false"}]`; `mtt in_progress t1` → `↩ compensating (1 command)` → `✗ git branch
  -D task/t1 (exit 1)`.
- **Why it matters:** s009 plans exactly this pattern on `session`'s `→ speccing` edge. Best-effort
  compensation correctly preserves exit 3, but the flagship rollback demo failing undermines the
  feature's credibility and strands agent state.
- **Fix sketch:** (a) change the documented pattern to leave the branch first:
  `rollback: git checkout - && git branch -D task/{{.ID}}` (the runner executes via the shell seam, so
  `&&` is fine — the coding template already relies on shell for `! make test`); (b) apply the same in
  the s009 dogfood config; (c) DESIGN.md + `docs/superpowers/notes/2026-07-09-flow-granularity-for-dogfood.md`
  carry the broken form — update both. Consider also the *idempotency* variant for retry-after-fix
  (`git switch -c … || git switch …`) — decide in the s009 brainstorm, don't mix into this fix.
- **Acceptance:** the repro above ends with the branch gone, `git symbolic-ref --short HEAD` back on
  the original branch, and the block error reporting `compensated 1 command` without `(1 failed)`.

### U2 — a blocked gate hides *why* it failed — SEVERITY: high

- **Symptom:** on block, the agent sees `✗ make lint (exit 2)` + `error: mtt: transition blocked by a
  failed gate: command "make lint" exited 2` — but not make's output. No hint that `-v` or
  `--log-file` exists. The agent's only move is to re-run the whole (slow) gate with `-v`.
- **Why it matters:** this is the single most frequent loop in dogfood (`make check` per `→ done`);
  every block costs a redundant full gate run. For an agent, the failure text IS the actionable context.
- **Fix sketch (either or both):** (a) cheap: append a hint to the CLI-layer block error — "re-run with
  -v or --log-file to see command output" (error text built around `core.ErrBlocked`,
  [internal/core/runner.go:29](internal/core/runner.go#L29); wording lives where the CLI formats it);
  (b) better: the exec Runner already captures output — retain the failing command's last ~10 lines and
  print them under the `✗` line by default (design intent "commands' own output is hidden by default"
  can stay true for *succeeding* commands).
- **Acceptance:** a blocked `mtt done` without flags shows either the failing command's output tail or
  an explicit `-v`/`--log-file` hint; e2e asserts one of them.

### U3 — `--json` is not honored where the agent needs it most — SEVERITY: medium

- **Symptom:** `mtt add "x" --json` prints `created t3` (prose; [internal/cli/add.go:75](internal/cli/add.go#L75))
  — the fresh ID must be parsed from text although a `taskJSON` view exists. `mtt show --json` omits
  `history` (human view renders it; the JSON consumer can't see checks/attribution at all).
- **Fix sketch:** on `--json`, `add` emits the created task via the shared `taskJSON`; extend
  `show --json` with a `history` array (entries: at/by/role/from/to/why/checks{cmd,exit}).
- **Acceptance:** `mtt add x --json | jq -r .id` yields the ID; `mtt show <id> --json | jq
  '.history[0].checks'` works after a gated move.

### U4 — discoverability gaps in the built-in help — SEVERITY: medium

- **Symptom:** (a) `mtt status --help` shows `Use: "status <id> <new-status>"`
  ([internal/cli/status.go:24](internal/cli/status.go#L24)) — but the id is optional (current-task
  resolution), and neither the root help nor `status --help` mentions the `mtt <status> [<id>]` verb
  sugar at all; (b) `error: no .mtt/ in "…"` ([internal/cli/project.go:28](internal/cli/project.go#L28))
  does not suggest `mtt init`.
- **Why it matters:** mtt's own adoption theory is "the agent learns from the CLI itself"; the sugar and
  current-task ergonomics are invisible to an agent that only reads `--help`.
- **Fix sketch:** `Use: "status [<id>] <new-status>"` + a `Long:` paragraph documenting sugar + current
  resolution; one line about the sugar in the root `Long:`; append `(run 'mtt init' to create one)` to
  the project.go error.
- **Acceptance:** `mtt status --help` and `mtt --help` mention both; the no-project error names `mtt init`.

### U5 — the binary's tagline undersells the killer feature — SEVERITY: low, one line

- **Symptom:** root `Short:` = "mtt — minimalist file-backed task tracker for agents and humans"
  ([internal/cli/root.go:22](internal/cli/root.go#L22)) — that's the crowded category (§3), not the
  empty niche. The first line an agent reads describes a commodity.
- **Fix sketch:** e.g. "mtt — executable task state machine for coding agents: status moves run gates
  and block on failure". Keep README/`Short:` consistent.
- **Acceptance:** `mtt --help | head -1` mentions gates/state machine; README tagline consistent.

### U6 — the coding template doesn't demo its own killer feature — SEVERITY: low (known e5_t6)

- **Symptom:** `coding` template's `tbd → in_progress` edges carry only the description ("create a
  feature branch") — no `git checkout -b feat/{{.ID}}` command, no rollback
  ([internal/adapter/yaml/templates/coding.yaml:17](internal/adapter/yaml/templates/coding.yaml#L17)).
  First-touch users never see a task-aware transition work.
- **Fix sketch:** land with e5_t6 (TASKS.md), using the **corrected** U1 rollback pattern. Note the
  template must stay valid for non-git dirs — acceptable: the gate simply fails there (same as `make`).
- **Acceptance:** fresh `mtt init --template coding` + `mtt in_progress f1` in a git repo creates the
  branch; a later gate failure compensates it away.

### U7 — documentation footnotes (no code change)

- `! make test` in the bugfix template passes when **make/Makefile is absent entirely** (any non-zero
  exit "proves" the failing test). Gates verify exit codes, not intent — one honest sentence in
  CLI_REFERENCE/DESIGN where gate semantics are described.
- An epic/phase whose children are all done stays `tbd`/open forever unless moved — expected (no
  auto-close by design), and the s009 self-referential gate (`! mtt list --parent {{.ID}} … | grep -q .`)
  is the intended answer; make sure dogfood docs say "closing a phase is a manual, gated move".

## 7. Priorities

| when | items | rationale |
|---|---|---|
| **before/with s009** | U1 (rollback pattern), U2 (blocked-gate visibility) | U1 is in the planned s009 config; U2's cost multiplies with every dogfood gate run |
| **pre-`v0.9.0` release chores** | R1 (vs-harness-hooks section), R2 (scan corrections), R3 (AGENTS.md snippet), U3, U4, U5, R0 | all cheap; they are the release's *positioning* surface |
| **later / with e5_t6** | U6, U7 | template demo rides the existing backlog item |

Cross-reference: s009 design inputs live in
`docs/superpowers/notes/2026-07-09-flow-granularity-for-dogfood.md` (decisions A/B) and
`docs/superpowers/specs/2026-07-09-session-009-dogfood-design.md`; U1 directly amends both.
