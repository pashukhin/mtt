# mtt config templates

Adaptable **samples**, not mandates — the engine is the product; a flow is
per-project data, and ours is a sample, not a mandate. Only `default.yaml` is
built into the binary (`mtt init`). The rest are examples you install with a
path or URL:

    mtt init --template ./templates/git-flow.yaml
    mtt init --template https://raw.githubusercontent.com/pashukhin/mtt/main/templates/git-flow.yaml

- `default.yaml`   — minimal flat flow (task + chore). The sole built-in.
- `coding.yaml`    — a dev flow (feature / bugfix / refactor).
- `hierarchy.yaml` — epics + tasks + subtasks.
- `git-flow.yaml`  — the git-integration flagship (branch → PR → deliver). Review
  its gate/post commands before use: it assumes a pushable `main`, `gh`+`jq`, and
  the repo's squash-merge title = PR title, and its state-moving edges carry the
  `--no-run` bypass caveat.

An installed external template is written **verbatim** (no `{{.Name}}`
substitution) and mtt's gate/post commands run via `sh -c` on your transitions —
review `.mtt/config.yaml` before your first move (config is code).
