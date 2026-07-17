# t29 — a home for pre-flow knowledge (no `mtt guide` command)

Status: spec (decision record). Type: task. Branch: `task/t29`.

## Context / problem

"Pre-flow knowledge" is what an agent needs *before* it starts moving tasks through the flow: how to
navigate the queue ("what do I do, in order?"), how to set up on a cold project ("first move"), and how to
resume ("I came back — where am I, what's next?"). Today this is scattered across agent-facing docs
(`AGENTS.md`, `NEXT_SESSION.md`) written for **this** repo's own dogfood, with no clear story for an
**adopting** project. The task title floats a `mtt guide` command as the possible home.

On inspection, the three scenarios are **already served** by existing surfaces:

- **queue navigation** → `mtt roadmap` (what to do, in order), `mtt ready` (unblocked), `mtt types` (the
  flows + gates), and the root-help "Start with …" pointer.
- **mid-flight resumption** → `mtt use` (the current-task pointer) + `mtt show` (status + the `▸` guidance
  and `next:` moves — the self-instructing flow guidance from t19).
- **first-move setup** → `README` (Install + a Quickstart that walks `init → add → status → done → list →
  tree → roadmap`) + the root-help orientation.

So the real question is **whether a runtime `mtt guide` command is warranted at all**, or whether the home is
the surfaces that already exist plus a small orientation gap.

Framing (settled with the maintainer): **approach A — no new command.** Setup is one-time-per-project, so it
belongs in static docs, not the software; navigation and resumption are already commands. t29's deliverable
is the **home decision** (which unblocks t23) plus one small orientation fix.

## Decisions

### D1 — Pre-flow knowledge has three homes; **no `mtt guide`/`resume` command**

| Home | Holds | Owner |
| --- | --- | --- |
| **`README`** (mtt's own repo) | *Learn the tool*: Install + the Quickstart loop (`init → add → flow → roadmap`). One-time read. | Already present; any polish is **t42** (docs audit). |
| **The `mtt init`-scaffolded runbook** | *How the adopter's agents work under mtt in their repo*: the maturation loop (`start → spec/plan/impl, each reviewed → deliver`), attribution (`MTT_BY` / `config.local` author), branch/PR mechanics. | Scaffolded by **t23** (which depends on t29 and inherits this split). |
| **root-help** (`mtt` / `mtt --help`) | Always-on in-tool orientation: **navigation** (`roadmap`/`ready`/`types`) **+ resumption** (`use`/`show`). | **t29** adds the resumption pointer (D2). |

### D2 — The one concrete change: a resumption pointer in the root-help `Long`

`internal/cli/root.go`'s root `Long` already orients a *fresh* start ("Start with `mtt roadmap` …
`mtt ready` … `mtt types` …") but never tells a **returning** agent how to re-orient. Add a resumption
clause so the in-tool orientation covers both directions. Target wording (final phrasing at implementation,
kept concise and within the existing sentence):

> … Start with `mtt roadmap` (what to do, in order), `mtt ready` (what is unblocked), `mtt types` (the flows
> and their gates); resuming, `mtt use` shows your current task and `mtt show` its status + next moves. All
> commands support --json.

This is the whole code change. It is the only real gap approach A leaves open — a returning agent not knowing
`use`/`show` is the resumption path — and it is squarely the in-tool orientation's job, not a new command.

### D3 — Why no command (the rejection, recorded)

- **DRY/YAGNI:** a `guide` would restate what `roadmap`/`ready`/`types` (navigation) and `use`/`show` + the
  t19 flow guidance (resumption) already do. Nothing new to compute.
- **Setup is one-time-per-project** → static docs (README) + the scaffolded runbook, not a runtime command an
  agent re-runs.
- A *state-aware* `mtt guide`/`resume` ("current: t1 [speccing]; next: submit → spec_review / cancel") was
  considered and declined: it is marginal over `mtt show` (which already prints status + `next:`), and the
  orientation gap it would fill is closed by D2 at a fraction of the surface.

## Scope / cross-refs

- **In:** this decision record + the D2 root-help resumption clause + a test asserting the help text.
- **Out:**
  - `README` "For agents" polish — that section is currently feature-boilerplate, not a work-loop
    orientation; tightening it belongs to **t42** (docs audit).
  - The per-project runbook scaffold — **t23** (`mtt init` extend), which depends on t29 and implements the
    runbook home this spec assigns to it.
  - Any `mtt guide` / `mtt resume` command — **rejected** (D3); not deferred.

## Acceptance criteria

1. The root-help `Long` names the **resumption** path (`mtt use` shows the current task; `mtt show` its status
   + next moves) alongside the existing navigation pointers — visible in `mtt --help` / `mtt help`.
2. A test asserts the help output contains the resumption pointer (e.g. `mtt use` **and** `mtt show` in the
   resumption clause), so the orientation can't silently regress.
3. No `mtt guide`/`resume` command is added; no other command's behavior changes.
4. `make check` green.

## Testing approach

- Unit (`internal/cli/root_test.go` or a focused help test): build `NewRootCmd()`, capture the root `Long`
  (or run `--help`), and assert it contains both the navigation pointers and the new resumption pointer
  (`mtt use` / `mtt show`). This pins the orientation contract D2 establishes without asserting exact prose.
