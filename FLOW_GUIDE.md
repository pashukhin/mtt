# Authoring flows in mtt

> Русская версия: [FLOW_GUIDE.ru.md](FLOW_GUIDE.ru.md). English is the source of truth; keep both in sync.

mtt is a **generic gated-flow engine**. A flow **binds the transitions of your entities to external
actions/artifacts**: when a thing moves from one status to the next, mtt runs the commands you hung on that
edge and only lets the move happen if they pass. The entity is anything — a task, a document, a purchase
request, an experiment, a release, a ticket. The action/artifact is anything — running a script, writing a
file, a git operation, an HTTP call, a notification. **Running a test suite is just one instance.**

A flow is **per-project data** — it lives in `.mtt/config.yaml`, which you author and commit. There are no
built-in flows and no default commands; you hang your own. This guide teaches the **engine**. Our own
development flow (branch → PR → deliver, walked through in §8) is shipped only as a **sample you adapt or
replace**, never as a mandate.

> **A note on emphasis.** mtt's headline pitch is the *agent gate* — "an agent can't declare `done` without
> passing your checks." That is the go-to-market framing; it does not narrow the engine, which is
> domain-neutral. Read this guide as "how to bind a status flow to real-world actions," with software
> development as one worked domain.

## 1. Mental model

Three layers, all name-agnostic (the engine has no hard-coded status or type names — they are all yours):

- **Types** — the kinds of entity you track (`task`, `post`, `request`, …). Each type has its own flow.
- **Statuses** — the states an entity of that type moves through, each with a **`kind`**: `initial`,
  `active`, or `terminal`.
- **Transitions** — directed edges between statuses. On an edge you may hang **`commands:`** (gates — all must
  pass or the move is blocked) and **`post:`** actions (run after the move is saved).

Start from a template (`mtt init`, or `mtt init --template coding|hierarchy`) and inspect any flow with
**`mtt types`**, which renders the graph, the gate commands, and the named edge verbs.

## 2. A minimal flow from scratch

The smallest **valid** flow has **three statuses — one of each kind**: `initial → active → terminal`. (A
two-status flow is rejected: every flow needs at least one `initial`, one `active`, and one `terminal` status.)

```yaml
version: 1
project: {name: demo}
types:
  - name: item
    prefix: i
    parents: []
    default: true
    statuses:
      - {name: todo,  kind: initial}
      - {name: doing, kind: active}
      - {name: done,  kind: terminal}
    transitions:
      - {from: todo,  to: doing}
      - {from: doing, to: done, commands: ["./gate.sh"]}
```

Write that to `.mtt/config.yaml`, and `mtt add "first item"` creates `i1` at `todo`; `mtt doing i1` moves it to
`doing`; `mtt done i1` runs `./gate.sh` and moves to `done` only if it exits 0.

## 3. The graph

- **Types & hierarchy.** `parents: []` is a root type. A type with `parents: [epic]` may be placed only under an
  `epic` (via `mtt add --parent <id>`); the hierarchy is defined entirely by `parents`. Mark exactly one type
  `default: true` — it is used by `mtt add` without `--type`. The `prefix` (letters only) is how the YAML
  adapter mints ids (`i1`, `t17`); it is an adapter detail, not part of the domain.
- **Statuses & `kind`.** `kind` is derived from the graph topology and validated against what you declare: an
  `initial` status has no incoming edge, a `terminal` has no outgoing edge, an `active` has both. You need at
  least one of each; you may have several `initial`/`terminal` statuses (e.g. a second terminal `cancelled`).
  When a flow has more than one `initial`, mark the entry one `default: true` (at most one; a `default` status
  must be `initial`).
- **Transitions & named edges.** Each transition has `from`/`to`. Give an edge a **`name`** to get a verb:
  `mtt do <id> <name>` (or the shorthand `mtt <name> <id>`) moves along the named edge out of the current
  status. Edge names must be unique per source status and disjoint from status names.

## 4. Gates: executable transitions

Hang **`commands:`** on a transition to gate it. They run in order; **all must exit 0**, or the move is
**blocked** (the entity stays put, exit code **3**). This is the core feature — the transition can run **any
external action**:

```yaml
      - {from: doing, to: done, commands: ["./gate.sh"]}          # any command
      - {from: in_progress, to: done, commands: ["make lint", "make test"]}   # one domain's instance
```

- **Placeholders.** `{{.ID}}`, `{{.Type}}`, `{{.From}}`, `{{.To}}` expand in commands (e.g.
  `git checkout -b task/{{.ID}}`). Only those four expand; a free-text field like `{{.Title}}` is a template
  error, never interpolated — a deliberate safety whitelist.
- **Timeouts.** Each command has a per-command budget: the global `command_timeout` (an adapter setting in
  `config.yaml`, **default `5m`**, overridable in `config.local.yaml`), or a per-command `timeout:` that
  overrides it. A long gate must raise it — e.g. `{run: 'make check', timeout: 10m}` — or the 5m default will
  SIGKILL it (and its whole process group).
- **Trust boundary.** A flow config runs **arbitrary shell with your privileges** — commands are trusted
  project config (like a Makefile or a git hook), never network input, and ids are charset-validated at load.
  Only run or commit a `.mtt/config.yaml` you trust, exactly as you would a `Makefile`.

## 5. Finalization: `post:` actions

A transition may also carry **`post:`** commands that run **after** the move is persisted — the finalization
phase (commit an artifact, push, open a PR, deploy, notify, archive). The two phases fail differently:

- a **gate** (`commands:`) failure **blocks** the move — nothing is persisted (exit **3**);
- a **`post:`** failure **keeps** the move — the status change is already saved; mtt exits **5** and prints the
  exact remaining `post:` commands so you finish them by hand. It never rolls back a persisted move.

**Rollback for side-effecting gates.** If a *gate* command has a side effect and a *later* gate command fails,
declare a **`rollback:`** compensator that undoes it (run in reverse over the commands that already succeeded).
This is an illustration — **our own dogfood config does not use rollback** (its start edge is idempotent, see
§8):

```yaml
      - from: tbd
        to: in_progress
        commands:
          - {run: 'git checkout -b task/{{.ID}}', rollback: 'git branch -D task/{{.ID}}'}
```

## 6. Attribution & guards

- **Who did it.** Every move records a `by`. It resolves in order: `--who`/`--by` (mutually exclusive) >
  `MTT_BY` (env) > `author:` in the gitignored `config.local.yaml` (your durable personal default). `--why`
  records a free-text reason.
- **Requiring attribution.** `require: {who: true, why: true}` — globally (a top-level config key) or per-edge —
  forces those fields *before* the gate runs; it is **tighten-only** (`config.local` and per-edge can add a
  requirement, never relax one). **This repo's committed config sets `require: {who: true}`,** so if you copy a
  config that requires `who` and set none of `--who`/`MTT_BY`/`author`, your very first move fails with **exit
  2**. Set your `author:` in `config.local.yaml` first.
- **Take-into-work pointer.** An edge may set `current: set` / `current: clear` (`Transition.Current`) to move
  the personal "current task" pointer, so later commands can omit the id (`mtt done` acts on the current one).

## 7. Flows for any domain

The engine is not dev-specific. The generic "script gate" is just §2 — the gate can be *any* command. Here are
two non-code flows.

**Content review** — a post moves draft → review → published, gated by a review script, published by a `post:`:

```yaml
version: 1
project: {name: blog}
types:
  - name: post
    prefix: p
    parents: []
    default: true
    statuses:
      - {name: draft,     kind: initial}
      - {name: review,    kind: active}
      - {name: published, kind: terminal}
      - {name: rejected,  kind: terminal}
    transitions:
      - {from: draft,  to: review,    name: submit,  commands: ["./review-checks.sh"]}
      - {from: review, to: published, name: publish, post: ["./publish.sh"]}
      - {from: review, to: rejected,  name: reject}
```

**Approval / sign-off** — a request is validated at the approve edge; a second terminal records a denial:

```yaml
version: 1
project: {name: procurement}
types:
  - name: request
    prefix: r
    parents: []
    default: true
    statuses:
      - {name: submitted, kind: initial}
      - {name: review,    kind: active}
      - {name: approved,  kind: terminal}
      - {name: denied,    kind: terminal}
    transitions:
      - {from: submitted, to: review}
      - {from: review, to: approved, name: approve, commands: ["./validate-request.sh"]}
      - {from: review, to: denied,   name: deny}
```

Runnable versions of these (`mtt init --template …`) ship separately — see the Neighbours section.

## 8. The git-integration pattern (a sample)

This repo's own committed `.mtt/config.yaml` is a full, working example: it binds the task flow to git so an
agent works in task terms while the flow hides the git mechanics. The moving parts:

- **`start` (tbd → …) creates the branch, idempotently:**
  `git switch task/{{.ID}} || (git switch main && git switch -c task/{{.ID}})`. It is re-entrant by
  construction, so it needs **no `rollback`** (unlike the §5 illustration).
- **Every move auto-commits `.mtt`** via a `post:` (`git add .mtt && git commit … -- .mtt`), so a move records
  its own state change.
- **`approve`** `post:` pushes the task branch and opens/updates the PR (`git push -u …`, then a `gh pr create`
  that is idempotent — skipped if an open PR exists).
- **`deliver`/`cancel`** run `git switch main` as their **first gate command** (so the terminal state lands on
  `main`), then their `post:` pushes `main`, with the add-pathspec **narrowed** to the task file (+ audit log)
  so a dirty `config.yaml` can't ride the main-landing commit past review.

**Honest caveats — this is opinionated.** It assumes GitHub + `gh` + `jq` + the `task/<id>` branch model + a
direct push to `main`. To adapt: swap `gh pr create` for a GitLab MR command (or drop it for a no-PR / trunk
flow); change the branch name; change or remove the main push if you protect `main` (branch protection breaks
the direct `deliver` push). The config also repeats its `post:` blocks heavily (the auto-commit block appears
24×, the main-push block 14×, across 12 `cancel` edges) — a global/default `post:` will let it be declared once
(that cleanup, and a one-command `mtt init` template for this flow, are tracked separately; see Neighbours).

## 9. Adaptation checklist

To adopt the git flow, copy this repo's `.mtt/config.yaml` and change:

1. **The gate command** — `make check` → your build/test/checks (need not be a Makefile).
2. **Artifact paths** — `docs/superpowers/pr/<id>.md` (PR body), spec/plan paths → your layout, or drop them.
3. **Tool deps** — the flow needs `gh` + `jq`; install them, or replace the PR-open command.
4. **Branch name** — `task/{{.ID}}` → your convention.
5. **`.gitignore`** — ensure `.mtt/.gitignore` ignores `config.local.yaml` (`mtt init` writes this for you).
6. **Attribution** — set `author:` in `config.local.yaml` (the committed config requires `who`; see §6).

## 10. Exercise your flow locally

Author test-first: change the config, then drive it and watch the gate.

```bash
# uses the §2 minimal flow (type `item`; todo → doing → done) as .mtt/config.yaml
mtt types                    # inspect the graph + gates + edge verbs (also validates the config)
mtt add "trial"              # creates i1 at the initial status `todo`
mtt doing i1                 # a gate-less move: todo → doing (the ▶/✓/✗ pipeline prints to stderr)
mtt done i1 -v               # runs the gate; -v (or --log-file gate.log) streams its output to debug a fail
mtt status i1 done --no-run --who me --why "skip"   # bypass the gate: --no-run is on `mtt status` only, and forces who+why
```

A blocked gate leaves the entity unchanged and echoes the failing command's output tail; re-run with `-v` or
`--log-file` to see everything.

**Authoring caveat — `--no-run` skips *every* command on the edge**, state-moving ones included (a
`git switch`, a deploy step): a bypassed move still writes the new status, but performs none of your edge's
actions — so a context-switching edge writes state wherever the tree currently is. mtt does not classify
your commands; they are yours, under execution and under bypass alike. Write edges knowing both paths exist
(the bypass is at least signed: `--no-run` forces `--who`+`--why` into history).

## 11. Validate your flow (structure)

`mtt add` and `mtt types` run `Config.Validate`, which rejects a structurally broken flow **before** runtime:

- every flow has ≥1 `initial`, ≥1 `active`, ≥1 `terminal` status, and `kind` matches topology;
- type and status names are unique; every transition `from`/`to` resolves to a real status;
- at most one `default` type, and at most one `default` status (which must be `initial`);
- edge `name`s are unique per source status and disjoint from status names.

(This is **structural** validation. *Reference* integrity — dangling `depends_on`, duplicate ids, dependency
cycles via `mtt check` — is a separate concern, strengthened in a later task; don't conflate the two.)

## 12. Neighbours

Out of scope here, tracked separately:

- **Runnable universal templates** — domain-neutral `mtt init --template <name>` samples (content-review,
  approval, generic script-gate) and the one-command version of the git flow above.
- **Agent-usage docs** — a generic tool-level `AGENTS.md`/`CLAUDE.md` for how an agent uses mtt day-to-day.
- **Settings & hooks** — scaffolding editor/agent settings and hooks (e.g. `sessionStart → mtt prime`).
