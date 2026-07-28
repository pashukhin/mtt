# templates

Repo-root package + directory holding mtt's init config templates as **plain files** (so they are
URL-fetchable: `raw.githubusercontent.com/…/templates/<name>.yaml`).

## Responsibilities

- `templates.go` embeds **only `default.yaml`** (`//go:embed default.yaml`) and exposes `Default() []byte` +
  `BuiltinNames()` (`{"default"}`). It lives at the repo root because `//go:embed` cannot reach a parent dir
  (`..`), so the embed must sit beside the file; `internal/adapter/yaml` imports it for the sole built-in.
- The other files are **hosted examples, NOT embedded** and NOT built-in names: `coding.yaml`,
  `docs.yaml`, `hierarchy.yaml`, and the **`git-flow.yaml` flagship** (a generalized, self-documenting derivative of the
  repo's dogfood flow — branch → agent review → human sign-off → PR → deliver, with every external assumption
  gated or stated in its edge descriptions). Install them with `mtt init --template <path|url>`.

## Invariants

- **`default.yaml` keeps `{{.Name}}`** — it is the built-in, rendered through `text/template` at init. The
  file examples (`coding`/`docs`/`hierarchy`/`git-flow`) are installed **verbatim** (no render), so they use a
  **literal** `project.name` — an unquoted `{{.Name}}` there would be invalid standalone YAML.
- `git-flow.yaml` derives from `.mtt/config.yaml`'s `task` type; its review-stage descriptions state the
  **machine-review → human-sign-off** ordering, matching the dogfood config (Goal 5/§8 of t62).

## Tests

`templates_test.go` (external test package `templates_test`): `TestExamplesLoadAndValidate` runs every hosted
example through `yaml.ValidateTemplateBytes` (NOT `default.yaml` — it carries `{{.Name}}`, valid only after
render); `TestDocsIsNoCode` asserts `docs.yaml` executes nothing (no `commands`/`post`/`post_defaults`/`events`)
— the structural form of its "no branch, no PR, no gate" promise; `TestGitFlowStatesItsAssumptions` asserts the flagship's descriptions mention `push`/`jq`/`--no-run`
+ the `gh` token + the "after the agent" ordering — so a future edit that drops a safety statement fails CI.
