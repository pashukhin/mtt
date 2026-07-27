# templates

Repo-root package + directory holding mtt's init config templates as **plain files** (so they are
URL-fetchable: `raw.githubusercontent.com/…/templates/<name>.yaml`).

## Responsibilities

- `templates.go` embeds **only `default.yaml`** (`//go:embed default.yaml`) and exposes `Default() []byte` +
  `BuiltinNames()` (`{"default"}`). It lives at the repo root because `//go:embed` cannot reach a parent dir
  (`..`), so the embed must sit beside the file; `internal/adapter/yaml` imports it for the sole built-in.
- The other files are **hosted examples, NOT embedded** and NOT built-in names: `coding.yaml`,
  `hierarchy.yaml`, the **`git-flow.yaml` flagship** (a generalized, self-documenting derivative of the
  repo's dogfood flow — branch → agent review → human sign-off → PR → deliver, with every external assumption
  gated or stated in its edge descriptions), and the **`github-gated.yaml` flagship** (t76: puts executable
  mtt gates on a GitHub issue's close — two linked entities; the issue's close is the task's terminal action
  `gh issue close`; drift is detect+complain via a `sync` self-loop, never enforced; needs `gh`+`jq`).
  Install them with `mtt init --template <path|url>`.

## Invariants

- **`default.yaml` keeps `{{.Name}}`** — it is the built-in, rendered through `text/template` at init. The
  file examples (`coding`/`hierarchy`/`git-flow`) are installed **verbatim** (no render), so they use a
  **literal** `project.name` — an unquoted `{{.Name}}` there would be invalid standalone YAML.
- `git-flow.yaml` derives from `.mtt/config.yaml`'s `task` type; its review-stage descriptions state the
  **machine-review → human-sign-off** ordering, matching the dogfood config (Goal 5/§8 of t62).

## Tests

`templates_test.go` (external test package `templates_test`): `TestExamplesLoadAndValidate` runs every hosted
example (now four: `coding`/`hierarchy`/`git-flow`/`github-gated`) through `yaml.ValidateTemplateBytes` (NOT
`default.yaml` — it carries `{{.Name}}`, valid only after render); `TestGitFlowStatesItsAssumptions` asserts
the flagship's descriptions mention `push`/`jq`/`--no-run` + the `gh` token + the "after the agent" ordering;
`TestGitHubGatedStatesItsAssumptions` asserts the github-gated flagship states `gh`/`jq`/`--no-run`/`drift`/
`author`/`bypass` (enforcement honesty + no silent traps) — so a future edit that drops a safety statement
fails CI.
