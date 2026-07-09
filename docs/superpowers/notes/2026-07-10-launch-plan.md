# Launch plan — when and how to start promoting mtt

Status: **decided direction**, 2026-07-10. Grounded in the market research of
[2026-07-09-positioning-and-agent-ux-analysis.md](2026-07-09-positioning-and-agent-ux-analysis.md)
(§1–5 + appendix). Russian digest of the whole analysis session:
[session summary (ru)](2026-07-10-analysis-session-summary.ru.md).

## Decision 1 — launch trigger: `v0.9.0` + the dogfood proof, NOT the first adapter

Do not wait for an external adapter (GitHub Issues / Trello / Jira). Rationale:

- Both channel precedents grew **without** tracker adapters: beads → ~25k★ on its local Dolt store,
  Backlog.md → ~6k★ on plain markdown. The initial audience (solo agent-devs; HN / r/ClaudeAI) buys the
  **zero-footprint + executable gates** story, not tracker integration.
- The niche window is narrowing (harness completion-hooks normalize through 2026); waiting for phase 8
  risks launching into an occupied slot.
- **Dogfood IS the launch asset**: "mtt's own development passes through its own gates" — a `history`
  excerpt with `checks` is the demo nobody else can show.
- Adapters follow demand: build the first one (**GitHub Issues** — that's where coding agents live;
  not Trello; bespoke Jira only paid/sponsored) when users ask — the second wave.

## Decision 2 — timing: no Monday deadline; gate the launch on readiness, timeboxed

**The question asked (2026-07-10): "do we have a week for polish, or must articles go out Monday
(3 days from now)?" Answer: there is no Monday deadline — and launching Monday would be a mistake.**

- **The window is measured in months, not days.** The squeeze (harness hooks, beads' "gate" vocabulary)
  is a 2026-trend, not an event next week; no competitor is known to ship per-type executable gates
  imminently. Three days of urgency buys nothing.
- **The channel is one-shot and punishes rough edges.** The beads backlash is the proof ("bugs confuse
  agents" killed retention; minimal replacements spawned). A Show-HN reader who hits today's known warts
  — a red gate with an invisible cause (U2), the broken documented rollback pattern (U1), no
  "vs harness hooks" answer in the README — does not come back for `v0.9.1`. First impressions in this
  channel do not get a second pass.
- **Launching now also means launching without the story.** Today mtt is `0.8.9-dev` with no dogfood
  proof; the pitch would be "another agent task tracker" — the saturated genre the research explicitly
  warns against. The un-copied core only lands with the self-host demonstration.
- **But polish must not stretch either.** The week is enough ONLY if spent on the defined release
  checklist, not new features: **s008.97 (hardening) → s009 (dogfood) → s009.5 (positioning) →
  tag `v0.9.0`** — two of the three are compact chores. **Timebox: if `v0.9.0` is not taggable in
  ~2 weeks, cut s009 scope (the simple session flow instead of the full decision-A flow) rather than
  slip the launch further.**

Practical schedule: skip Monday; target the article drop for the week `v0.9.0` is tagged. If a
concrete external reason to pre-announce exists (it is not known to this analysis), a teaser post can
precede the tag — but the Show-HN moment must coincide with a working, self-hosting release.

## Launch assets (in priority order)

1. **The AGENTS.md adoption snippet** in README (R3 — drafted in the positioning note §4): the actual
   conversion artifact; readers install what their agent can start using in one paste.
2. **Write-up: "How mtt gates its own development"** — the dogfood story with a real `history` excerpt
   (checks, who/why), the `session` flow diagram, and one honest "what it doesn't do" section
   (cooperative discipline, not a jail; `--no-run` exists and is signed).
3. **README positioning surface** — the "why not harness hooks?" section (R1), refreshed competitive
   scan (R2), tagline that names the gate feature (U5).
4. **Show HN + r/ClaudeAI posts** — lead with the enforcement wedge ("an executable task state machine:
   your agent can't say done until the gate is green"), not with "task tracker". Cite the 2026
   problem-validation evidence (75%-rework issue, false-positive benchmarks) sparingly — one number.
5. Secondary channels after the first wave reaction: lobste.rs, dev.to, a comparison post vs beads /
   Backlog.md / harness hooks (the honest table from the positioning note §2).

## Success/abort signals for wave 2 (the adapter decision)

- Signal to build the GitHub Issues adapter: recurring "can it write to my tracker?" asks from real
  users (issues/comments), not speculation.
- Signal the pitch needs work instead: "how is this different from Claude Code tasks/hooks?" dominating
  the thread — means R1 landed weakly; iterate the positioning, not the feature set.
