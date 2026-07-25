# t62 — Runnable init templates (spec)

Status: revision 1 — decided in the 2026-07-25 brainstorm (three user decisions: template strategy,
built-in scope, external-source safety UX; + one user refinement: a top-level `templates/` directory).

## Problem

t62 was split from t57 (2026-07-23 brainstorm): t57 shipped the flow-authoring **GUIDE** (docs-only);
t62 ships the **runnable, discoverable** side. Today `mtt init --template <name>` renders one of three
**embedded** configs (`default`/`coding`/`hierarchy`, `text/template` `{{.Name}}` substitution). Two gaps:

1. There is no way to start from a flow the user (or we) authored **outside** the binary — every template
   must be compiled in, which grows a maintenance surface and reads as a prescribed menu, against the
   "the engine is the product; a flow is per-project data; our flow is a sample, not a mandate" framing (t57).
2. The **git-integration flagship** (our dogfood flow: branch → auto-commit → PR → deliver) was deferred
   until t24 collapsed the duplicated `post:` blocks; **t24 is now resolved by t66** (`post_defaults` +
   `inherit_post: false`), so the flagship is unblocked — but embedding it would carry a heavy
   `{{.ID}}`-templating carve-out and per-edge safety duties into the binary.

## Decisions (brainstorm record, 2026-07-25)

1. **Strategy = one minimal built-in + bring-your-own** (user decision, Option 3 of 3): ship a single
   embedded template and support `mtt init --template <path|url>` (local file **or** URL). Rejected: a
   curated embedded sample set (maintenance surface, reads as a menu); minimal-only without an external
   path (no real starting flows).
2. **Built-in scope = `default` only** (user decision): keep today's `default` (task+chore flat flow) as the
   **sole** embedded, offline, no-network built-in. `coding`/`hierarchy` are **de-embedded** (not deleted —
   see Decision 4). No embedded git flagship.
3. **External-source safety UX** (user decision): a local **path** installs prompt-free; a **URL** additionally
   requires an interactive **y/N confirmation** (`--yes` skips it for scripts). Both **validate before write**
   (fail-closed) and print a **loud SEC2 review notice**. `init` never executes template commands — the risk
   window is the user's first transition, guarded by the notice.
4. **A top-level `templates/` directory** (user refinement): the existing config templates **plus our own**
   git flow live as plain files in a repo-root `templates/` directory, so they are **URL-fetchable** via
   `raw.githubusercontent.com/…/templates/<name>.yaml`. Only `default.yaml` is **embedded**; the rest are
   hosted examples (not compiled in, not built-in names). This preserves `coding`/`hierarchy` as fetchable
   examples and gives the git flagship a clean home that is **not** the live operational `.mtt/config.yaml`.
5. **External templates are written verbatim** — no init-time `{{.Name}}` templating for path/url sources: a
   real config carries `{{.ID}}`/`{{.Type}}` **runtime** placeholders (expanded by `core` at transition time)
   that init-time `text/template` (`missingkey=error`) would choke on. Only the embedded `default` is
   `{{.Name}}`-templated. External is your config-as-code, byte-for-byte.

## Goals

1. `mtt init --template <arg>` accepts three source kinds — a **built-in name** (`default`), a **local file
   path**, or an **https URL** — resolved by an unambiguous rule.
2. **No silent traps** (release bar, 2026-07-24): every external assumption is gated or stated. The URL
   fetch is https-only + confirmed + capped; an untrusted config is validated + written-not-run + announced.
3. Ship the **git-integration flagship** as `templates/git-flow.yaml` — a generalized, self-documenting
   derivative of our dogfood flow whose edge descriptions already carry the pushable-main (t33), gh+jq,
   squash-title, and `--no-run` bypass (t32) statements. It is a shipped template and meets the release bar.
4. Preserve test coverage and the FLOW_GUIDE forward-link; docs (EN+RU) reflect the new `--template` forms.

## Non-goals

- **A curated embedded sample set** — rejected in Decision 1. The domain-neutral content-review / approval /
  generic-script-gate trio (t57 Part 1 candidates) is **not** shipped; `default` + the hosted examples
  (`coding`/`hierarchy`/`git-flow`) suffice to demonstrate generality. (Revisit only if a real need appears.)
- **Executing template commands at init** — `init` only writes config; it never runs a gate/post command.
- **Auth / private template sources** — URL fetch is anonymous https of a public file (no tokens, no auth
  headers). Private hosting is out.
- **A template registry / index / discovery service** — `--template <url>` is a direct file fetch.
- **Non-https remote** — `http://` is refused (Decision 3); no ftp/file:// schemes.

## Design

### 1. Top-level `templates/` directory + embed policy

A new repo-root `templates/` directory holds the config templates as plain YAML files:

```
templates/
  default.yaml       # the sole EMBEDDED built-in (task+chore flat flow; {{.Name}})
  coding.yaml        # hosted example (de-embedded)
  hierarchy.yaml     # hosted example (de-embedded; also the roadmap e2e fixture)
  git-flow.yaml      # the git-integration FLAGSHIP (generalized dogfood flow)
  README.md          # "adaptable samples, not mandates"; how to fetch via --template <url>
```

Only `default.yaml` is compiled in. Because `//go:embed` cannot reach a parent directory (`..` is
rejected), the embed lives in a **root-level `templates` Go package** (`templates/templates.go`,
`//go:embed default.yaml`) exposing the built-in bytes + the valid built-in names; `internal/adapter/yaml`
imports it. The other files sit in the directory un-embedded — present in a checkout and fetchable by URL,
never bloating the binary. A golden/round-trip test asserts `default.yaml` loads + validates.

### 2. `--template <arg>` resolution rule (pure, tested)

Classify `arg` in this order (built-in names win for bare tokens; explicit shape selects file/url):

1. prefix `https://` → **URL** source. A `http://` prefix is a hard error ("remote templates must be https").
2. contains a path separator **or** ends in `.yaml`/`.yml` → **local file path** (a path that does not
   exist / is unreadable is a clear error, exit 1). So a file is selected only by an explicit path shape.
3. otherwise (a **bare token** — no separator, no yaml suffix) → **built-in name**: `default` resolves to the
   embedded built-in; any other bare name is the existing sorted-valid-names error (never silently a file).

Bare tokens are thus reserved for built-in names — `--template default` is the built-in even if a file
`default` exists in cwd; a user selects that file with `./default` or `default.yaml`. The classifier is a
pure function (returns the source kind + cleaned value) so the CLI can decide whether a confirmation prompt
is needed before any I/O.

### 3. External source flow (path or URL)

Both external kinds share: **fetch → validate → (confirm, url only) → write-verbatim → notice**.

- **Fetch bytes.** File: read the path. URL: `GET` behind an injectable seam (see §5) — **https-only**, a
  **~30s timeout**, a **~1 MiB** body cap (`io.LimitReader`; over-cap is an error), redirects followed but
  the final hop must stay https (a redirect to http is refused).
- **Validate before write (fail-closed).** Parse the bytes through the config load path and run
  `Config.Validate` (the same checks `Load` runs — exactly one default type, prefix rules, flow sanity). A
  template that does not parse/validate errors and **writes nothing** (no partial `.mtt/config.yaml`).
- **URL confirmation.** A URL source prompts on stderr: `install template from <url>? [y/N]` reading
  `cmd.InOrStdin()`; only `y`/`yes` proceeds. `--yes` skips the prompt. **Non-interactive guard:** if stdin
  is **not a TTY** and `--yes` was not passed, the URL is **refused** (exit 1, "refusing to fetch a remote
  template without confirmation; pass --yes") — no silent remote fetch in a pipe/CI. A local path never
  prompts.
- **Write verbatim.** External bytes are written **as-is** (Decision 5) via the existing atomic write; no
  `{{.Name}}` substitution. The embedded `default` keeps its `{{.Name}}` render path.

### 4. SEC2 "loud" review notice

After a successful **external** write (path or url), print a prominent notice on stderr:

```
⚠  installed .mtt/config.yaml from <source>
   this config's gate/post commands run via `sh -c` on your transitions.
   review .mtt/config.yaml before your first `mtt` move.
```

`init` runs nothing, so the guard is review-before-first-transition. The built-in `default` (no commands)
prints the normal `initialized …` line only. `--json` gains a `source` field (`builtin:default` |
`file:<path>` | `url:<url>`) so an automated caller sees provenance; the notice text is stderr-only.

### 5. Architecture / testability

- **Classification** (§2) is a pure function — unit-tested table.
- **Network fetch** is behind an injectable seam (mirroring t44's `ReleaseSource` → `internal/adapter/github`):
  a small `TemplateFetcher` (`Fetch(url) ([]byte, error)`) with an https implementation and a fake/`httptest`
  in tests. **No external network in tests** (AGENTS.md): URL paths are unit-tested against a loopback
  `httptest.Server` (deterministic); e2e covers built-in + file-path + the **non-TTY URL refuse** (which
  errors before any fetch, so it needs no server).
- **Validation** reuses the existing config parse + `Config.Validate` — no second validator.
- The **CLI stays thin**: classify → (prompt on url) → resolve bytes → validate → write → notice/JSON. The
  adapter owns fetch/read/validate/write; `core`/`pkg` own no new policy beyond what `Config.Validate`
  already enforces.

### 6. The git-flow flagship (`templates/git-flow.yaml`)

A generalized, project-neutral derivative of the repo's dogfood flow (one task type, the
brainstorm→spec→plan→TDD→review→deliver lifecycle, git `post:` actions via `post_defaults` +
`inherit_post: false`). It is a **shipped template**, so it meets the release bar in its own content:

- **Stated or gated external assumptions** (no silent traps): pushable `main` (t33) — stated in the deliver
  edge description; `gh`+`jq` presence — stated where `approve`/`deliver` use them; `squash_merge_commit_title
  = PR_TITLE` — stated where the deliver grep relies on it; the `--no-run` bypass caveat (t32) — in each
  state-moving edge's own description (a bypass skips **all** commands incl. context moves; the client owns
  their commands under bypass exactly as under execution).
- It loads + validates (test). It is the canonical target of `mtt init --template <url>` and the FLOW_GUIDE
  forward-link. It is **not** the live `.mtt/config.yaml` (which stays repo-operational).

### 7. Removed built-ins + test/migration

- `renderTemplate`'s built-in registry shrinks to `{default}`; `--template` help + the unknown-name error
  list only `default`.
- `coding`/`hierarchy` move to `templates/` as hosted examples. Tests that used them migrate to the
  **file-path** source (also exercising §2/§3): `roadmap.txt` (needs epic/parent types) ships `hierarchy.yaml`
  as a txtar file in `$WORK` and does `mtt init --template ./hierarchy.yaml`; `init_test.go`/`init.txt` update
  their template list + assertions.

## Testing (TDD)

- **Classification** unit table: `default`→builtin; `x.yaml`/`./x`/`a/b`→file; `https://…`→url;
  `http://…`→error; unknown bare→sorted-names error.
- **Validate-fail-closed**: a malformed external template errors and writes no `.mtt/config.yaml`.
- **Verbatim**: an external template containing `{{.ID}}` survives byte-for-byte (not eaten / not an error).
- **URL (httptest, loopback)**: success; https-reject (`http://`); size-cap; timeout; redirect-to-http refuse;
  `--yes` skips confirm; non-TTY + no `--yes` → refuse (no fetch).
- **Confirm**: `y` proceeds, `n`/empty aborts (no write).
- **Notice + `--json` source**: external write prints the ⚠ block on stderr and sets `source`; built-in does
  not.
- **Built-in `default` unchanged**: existing golden/e2e still green; `git-flow.yaml` + `default.yaml` load +
  validate.
- **e2e**: `init.txt` (built-in + file-path + non-TTY-url-refuse), migrated `roadmap.txt`.

## Docs

Sweep all parallel occurrences (EN+RU) per the design-docs-parallel-occurrences lesson:

- **CLI_REFERENCE (EN+RU):** `mtt init --template <name|path|url>` — the three forms, the resolution rule, the
  https-only + confirm + `--yes` + caps for URL, validate-before-write, the review notice, the `--json`
  `source` field; the built-in list is now just `default`.
- **FLOW_GUIDE (EN+RU):** the forward-link resolves — "install a ready flow: `mtt init --template <url>`",
  pointing at `templates/` (the git-flow flagship named).
- **DESIGN (EN+RU):** a Shipped(t62) note — the minimal-built-in + bring-your-own strategy, the top-level
  `templates/` dir, the no-silent-traps / SEC2 posture, t24→t66 unblocking the flagship.
- **CLAUDE.md:** `internal/cli` (init external-source flow + confirm/notice), `internal/adapter/yaml`
  (classification/fetch/validate/verbatim), and the new root `templates` package.
- **CHANGELOG:** one `[Unreleased]` entry. **`templates/README.md`:** adaptable-samples framing + fetch how-to.

## Open seams (recorded, not built)

- **`--template <url>` for private/authed sources** — anonymous https only here; tokens/auth are a later ask.
- **Template index / discovery** — no registry; a direct file fetch only.
- **URL happy-path in `testscript`** — covered by unit `httptest`; a harness-served loopback URL for e2e is
  possible (start a server in `TestMain`, pass via env) but deferred unless the unit coverage proves thin.
- **A checksum/pin for a fetched template** — out of scope; the loud notice + validate-before-write is the
  trust boundary (config-as-code is reviewed like a Makefile, SEC2).
