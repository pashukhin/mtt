# t19 — implementation plan (flow-led process)

Spec: `docs/superpowers/specs/t19-flow-led-process.md` (approved). Change surface: `.mtt/config.yaml`
descriptions + guard-test spot-checks + AGENTS.md + root CLAUDE.md. No production code. Branch
`task/t19` (flow-managed). `make check` green before every commit.

## Task 1 — guard spot-checks (red) → D1 description texts (green)

1. In `internal/adapter/yaml/dogfood_test.go`, next to the existing description spot-checks
   (`it must be a`, `pull main`), add:

```go
	if s, _ := task.StatusByName("speccing"); !strings.Contains(s.Description, "superpowers:brainstorming") {
		t.Fatalf("task speccing description lost the brainstorm step: %q", s.Description)
	}
	for _, tc := range []mtt.Type{task, chore} {
		if s, _ := tc.StatusByName("approved"); !strings.Contains(s.Description, "gh pr create") {
			t.Fatalf("%s approved description lost the PR command: %q", tc.Name, s.Description)
		}
	}
```

2. Run `go test ./internal/adapter/yaml/ -run TestRepoDogfoodConfig` → FAIL (speccing lacks the string).
3. Apply the D1 texts to `.mtt/config.yaml` — replace these description values verbatim from the spec.
   Each value is ONE line inside the existing double-quoted YAML scalar; literal backticks stay exactly
   as in the current config (the plan's markdown delimiters are not part of the value):
   - task `speccing`: `brainstorm first (superpowers:brainstorming), then write the design spec — a
     decision record — to docs/superpowers/specs/<this-task-id>-<slug>.md (commit early and often),
     then `mtt submit``
   - task `planning`: `write the implementation plan (superpowers:writing-plans) to
     docs/superpowers/plans/<this-task-id>-<slug>.md, then `mtt submit``
   - task `impl_review`: `run an adversarial code review: the AGENTS.md Principles self-check + Go
     conventions, and DESIGN.md/CLAUDE.md updated if behavior changed; `mtt approve` when it passes,
     `mtt decline` to send back`
   - chore `impl_review`: `run an adversarial code review: the AGENTS.md Principles self-check + Go
     conventions, and DESIGN.md/CLAUDE.md updated if behavior changed; if the diff contains design
     decisions not recorded elsewhere — decline: it must be a `task` (cancel this chore and recreate).
     `mtt approve` / `mtt decline``
   - `approved` (both types): `open/update the PR: gh pr create --title '<this-task-id>: <title>' (the
     branch was auto-pushed), ask the human to merge; after the squash-merge run `mtt deliver`;
     human-requested changes -> `mtt decline``
4. Test → PASS. `make check` → OK. Commit: `t19: flow descriptions carry the method (D1) + guard spot-checks`.

## Task 2 — AGENTS.md (D2)

1. **DoD section**: replace the checklist body with: the DoD is the flow — each status prints its
   instructions on entry and in `mtt show` (`mtt types` shows the type + edge map); what remains on the
   agent: test-before-code, the Principles self-check, docs-sync judgment.
2. **Working under mtt** — cut to pointers (the flow prints these): the two-type litmus sentence (keep
   one line: "pick the type by its description — `mtt types`"), the artifacts bullet, the delivery
   bullet (this deletes the stale approved-push sentence; keep one pointer line "delivery is verified —
   the deliver edge explains; the PR-title→squash propagation rationale is in DESIGN.md's dogfood
   note"), and the **"Move by edge verb" bullet** — cut entirely EXCEPT its last sentence (mid-flight
   resumption), which moves to the keep list; the rest (edge names, branch/tree mechanics) is what the
   edge descriptions print. KEEP: backlog navigation, attribution setup, mid-flight resumption,
   dangerous-ops summary, auto-commit/auto-push + exit-5 recovery, config-is-code.
3. **Sessions section**: rewrite — the unit of work is an mtt task on a flow-created `task/<id>` branch;
   method steps live in the flow; `sessions/*.md` is a narrative archive for process milestones (its
   future is t31), not a per-task requirement.
4. **Git section**: `Branches:` bullet → flow-created `task/<id>` for all mtt work; `feat/…`, `fix/…`,
   `chore/…` remain for non-task exceptions (bootstrap/infra) — all three kept, deliberately (ratify at
   plan_human_review). **"Small commits, imperative mood." is KEPT** in the rewritten bullet (discipline,
   not expressible in the flow).
5. `make check` → OK. Commit: `t19: AGENTS.md — discipline+principles only, flow leads the process (D2)`.

## Task 3 — root CLAUDE.md (D3)

Header: "task plan — in TASKS.md" → "the live queue — `mtt roadmap` (TASKS.md is frozen history)";
reading order: `AGENTS.md → DESIGN.md → mtt roadmap`; non-negotiables bullet gains "(the flow creates
and pushes the branch; see AGENTS.md 'Working under mtt')". `make check` → OK. Commit:
`t19: CLAUDE.md — queue lives in mtt (D3)`.

## Task 4 — submit

`mtt submit t19` (the edge runs `make check` itself) → impl_review → adversarial code review per the
(new) status instruction → approve/decline per findings.

## Acceptance (from the spec)

Move into touched statuses / `mtt show` prints the D1 texts; guard green incl. all description
spot-checks (old two + new: `superpowers:brainstorming`, `gh pr create` ×2 types); AGENTS.md
per D2 with no sentence contradicting the flow's printed guidance; CLAUDE.md per D3; `make check` green.
