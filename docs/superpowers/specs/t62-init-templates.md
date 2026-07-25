# t62 — Runnable init templates (spec)

Status: revision 3 — rev2 addressed the 2026-07-25 adversarial spec review (1 blocker, 4 majors, 5 minors);
rev3 fixes the 3 issues that review introduced (the `Load`/overlay boundary MAJOR + 2 minors). Decided in the
2026-07-25 brainstorm (three user decisions: template strategy, built-in scope, external-source safety UX;
+ one user refinement: a top-level `templates/` directory).

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
- **SSRF / loopback / internal-address blocking** — the user runs the fetch on their own machine against a URL
  they typed; guarding against loopback/metadata-endpoint targets is out of scope (it would also foreclose a
  future loopback test server). https-only + the loud review notice + validate-before-write is the boundary.
- **A checksum / signature pin for a fetched template** — config-as-code is reviewed like a Makefile (SEC2);
  the trust boundary is the notice + validate-before-write, not a supply-chain pin.

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
2. contains a path separator (`/` **or** the OS `filepath.Separator`, so Windows `\` counts) **or** ends in
   `.yaml`/`.yml` → **local file path**. A path that does not exist / is unreadable is a clear error (exit 1);
   when the missing path *looks* like a scheme-less URL (a `.`-bearing host segment before the first `/`, e.g.
   `raw.githubusercontent.com/…/x.yaml` — but **not** a leading `./`/`../` relative path, whose first segment
   is just `.`/`..`), the error appends a hint: "did you mean `https://…`?".
3. otherwise (a **bare token** — no separator, no yaml suffix) → **built-in name**: `default` resolves to the
   embedded built-in; any other bare name is the existing sorted-valid-names error (never silently a file).

Bare tokens are thus reserved for built-in names — `--template default` is the built-in even if a file
`default` exists in cwd; a user selects that file with `./default` or `default.yaml`. The classifier is a
pure function (returns the source kind + cleaned value) so the CLI can decide whether a confirmation prompt
is needed before any I/O. (SSRF / loopback-address blocking on a user-supplied URL is a **non-goal** — the
user runs the fetch on their own machine against a URL they typed; see Non-goals.)

### 3. External source flow (path or URL)

Both external kinds share: **(confirm, url only) → fetch → validate → write-verbatim → notice**. The
confirm precedes the fetch so a declined URL touches the network zero times.

- **URL confirmation (before any I/O).** A URL source prompts on stderr: `install template from <url>?
  [y/N]` reading `cmd.InOrStdin()`; only `y`/`yes` proceeds. `--yes` skips it. **Non-interactive guard:** if
  stdin is **not a TTY** and `--yes` was not passed, the URL is **refused** (exit 1, "refusing to fetch a
  remote template without confirmation; pass --yes") — no silent remote fetch in a pipe/CI. A local path never
  prompts. The decision is a **pure function** `confirmRemote(in io.Reader, isTTY, autoYes bool) (proceed bool,
  err error)` — unit-tested directly (isTTY=true + a scripted buffer for the `y`/`n`/empty paths; the
  non-TTY-refuse path with isTTY=false). **TTY detection uses the stdlib** — `os.Stdin.Stat()` &
  `os.ModeCharDevice` — so **no new dependency** (avoids `golang.org/x/term`; keeps the `go 1.23.1` floor per
  the go-get-@latest lesson).
- **Fetch bytes.** File: `os.ReadFile`. URL: `GET` behind an injectable **transport** seam (see §5) —
  **https-only**, a **~30s timeout**, a non-`200` response is an error (`fetch <url>: HTTP <status>`, so a
  404/500 reads as "not found", not "invalid config"), redirects followed but a redirect whose target is not
  https is **refused** (an `http.Client.CheckRedirect` = `httpsOnlyRedirect` that rejects a non-https hop).
  **Size cap:** read with `io.LimitReader(body, cap+1)` and error when the result exceeds `cap` (~1 MiB) —
  `LimitReader` truncates **silently**, so the `+1` sentinel is required to detect (not silently truncate) an
  over-cap body.
- **Validate before write (fail-closed).** Factor the adapter's **post-decode** checks into a shared helper
  `checkDecoded(yc ymlConfig) error` — `toDomain` + **`checkPrefixes`** (exactly-one-default, prefix present +
  unique + **letters-only**, the documented **shell-safety boundary**: a minted `<prefix><N>` is expanded into
  `{{.ID}}` inside `sh -c`, so a prefix with shell metacharacters must be refused at init, not deferred to the
  first move) + **`parseCommandTimeout`** + `Config.Validate` (topology/flow/parents/events). The external path
  is a thin `ValidateTemplateBytes(data)` = decode a **single** YAML doc → `checkDecoded`. **`Load` is NOT
  rewritten as a one-slice decode** — it keeps its **two-file overlay** (`config.yaml` + the gitignored
  `config.local.yaml`, later-wins), its **pre-overlay `committedRequire` capture** and **tighten-only
  `require`** (committed || local), and `Author` from the overlay; it merely routes its checks through the same
  `checkDecoded`. So the file config and an external template get the **same check logic** — *not* the same
  inputs: the file path legitimately has the extra overlay input. `Config.Validate` alone is **insufficient**
  (no prefixes/timeout/single-default). A template failing any check errors and **writes nothing** (no partial
  `.mtt/config.yaml`).
- **Write verbatim.** External bytes are written **as-is** (Decision 5) via the existing atomic write; no
  `{{.Name}}` substitution. The embedded `default` keeps its `{{.Name}}` render path. Consequently `--name` is
  **ignored for external sources** (verbatim skips `renderTemplate`): the CLI prints a stderr note when `--name`
  is passed with a path/url so the drop is never silent, and `git-flow.yaml` pins a literal neutral
  `project.name` (not `{{.Name}}`, which `core` never expands — it only expands `{{.ID}}/{{.Type}}/{{.From}}/
  {{.To}}` in commands).

### 4. SEC2 "loud" review notice

After a successful **external** write (path or url), print a prominent notice on stderr:

```
⚠  installed .mtt/config.yaml from <source>
   this config's gate/post commands run via `sh -c` on your transitions.
   review .mtt/config.yaml before your first `mtt` move.
```

`init` runs nothing, so the guard is review-before-first-transition. The built-in `default` (no commands)
prints the normal `initialized …` line only. `--json` gains a `source` field (`builtin:default` |
`file:<path>` | `url:<url>`) so an automated caller sees provenance; the existing `template` field stays
(now holding the raw `--template` arg — the built-in name, path, or url) for back-compat with `init.txt`'s
`"template": "default"` assertion. The notice text is stderr-only, so it never pollutes `--json` stdout.

### 5. Architecture / testability

- **Classification** (§2) is a pure function — unit-tested table.
- **Network fetch** keeps t44's **no-socket** discipline but seams at the **`http.RoundTripper` (transport)**
  layer, not `httpDoer`, so redirect handling is *real*: a real `Fetcher{client *http.Client}` whose client has
  `CheckRedirect: httpsOnlyRedirect` + a ~30s `Timeout`; the injectable seam is the client's **`Transport`**.
  Unit tests install a **fake transport** that serves crafted `*http.Response`s per request (including a `30x`
  then a second-hop response) with **no socket**, so the *real* client redirect loop invokes
  `httpsOnlyRedirect`. (A fake `httpDoer`/`Do` — the github precedent — **replaces** the client and thus never
  runs `CheckRedirect`; t44 gets away with it because it does not test redirects. We do, so the seam must be
  the transport, or the predicate tested purely — we do **both**.) Branches: https-reject (scheme check
  pre-request), **redirect-to-http refuse** (fake transport serves a `302`→http; real `CheckRedirect` rejects
  it) **plus** `httpsOnlyRedirect(req, via) error` as a pure unit test, HTTP 404/500, over-cap body, success —
  all hermetic, honoring AGENTS.md "no network in tests".
- **Confirm/TTY** is a pure `confirmRemote(in, isTTY, autoYes)` (§3) — the *decision* is unit-tested with a
  scripted reader for `y`/`n`/empty/`--yes`/non-TTY; only the thin TTY-detection call (`os.Stdin.Stat()`) sits
  in the CLI, uninjected (its non-TTY result is what the e2e subprocess exercises).
- **Validation** is the shared `checkDecoded(yc)` (§3): `Load` calls it **after** its two-file overlay; the
  external path calls it via `ValidateTemplateBytes(data)` on a single decoded doc. One check-set, two callers
  — no second/weaker validator, and `Load`'s overlay / tighten-only `require` / `Author` semantics are
  **untouched** (a regression test pins that).
- **Coverage split (honest):** the real binary in `testscript` runs as a non-TTY subprocess, so e2e covers
  **built-in `default`**, **file-path install** (+ the verbatim/`{{.ID}}` survival, validate-fail-closed), and
  the **non-TTY URL refuse** (errors before any fetch — no server needed). The **URL happy path + fetch error
  branches** and the **interactive confirm** are **unit-tested** (fake transport / scripted `confirmRemote`),
  since a testscript subprocess can neither present a TTY nor be handed a fake doer. This split is deliberate
  and stated (not an overclaim).
- The **CLI stays thin**: classify → (confirmRemote on url) → fetch (adapter) → `ValidateTemplateBytes`
  (adapter) → write (adapter) → notice/JSON. The adapter owns fetch/read/decode-validate/write; `core`/`pkg`
  own no new policy.

### 6. The git-flow flagship (`templates/git-flow.yaml`)

A generalized, project-neutral derivative of the repo's dogfood flow (one task type, the
brainstorm→spec→plan→TDD→review→deliver lifecycle, git `post:` actions via `post_defaults` +
`inherit_post: false`). It is a **shipped template**, so it meets the release bar in its own content:

- **Stated or gated external assumptions** (no silent traps): pushable `main` (t33) — stated in the deliver
  edge description; `gh`+`jq` presence — stated where `approve`/`deliver` use them; `squash_merge_commit_title
  = PR_TITLE` — stated where the deliver grep relies on it; the `--no-run` bypass caveat (t32) — in each
  state-moving edge's own description (a bypass skips **all** commands incl. context moves; the client owns
  their commands under bypass exactly as under execution).
- **The "stated" half is made testable, not just reviewer-trusted:** a Go test asserts `git-flow.yaml`'s edge
  descriptions actually contain the required safety keywords — `push`, `jq`, `--no-run`, and `gh` matched as a
  **token** (word-boundary / backticked `` `gh` ``, **not** the bare bigram `gh`, which occurs in
  "throu**gh**"/"hi**gh**"/"ri**gh**t") — so a future edit that drops a safety statement fails the suite. The
  template also **loads + validates**.
- **Explicit scope reduction (owned, not silent):** the release bar says templates "ship runnable and tested
  like `demo/`". A git-integration flow **cannot** run hermetically (it needs a real remote, `gh`, a merged
  PR), so `git-flow.yaml` ships **load+validate + the keyword test**, **not** a `coding-flow.sh`-style runnable
  demo. This is a deliberate, stated carve-out — the runnable-demo bar applies to command-free sample flows,
  which this task does not add.
- It is the canonical target of `mtt init --template <url>` and the FLOW_GUIDE forward-link. It is **not** the
  live `.mtt/config.yaml` (which stays repo-operational).

### 7. Removed built-ins + test/migration

De-embedding `coding`/`hierarchy` removes them as **built-in names**, so every `--template coding|hierarchy`
call site must migrate. The rev1 spec undercounted this badly (spec-review MAJOR + BLOCKER); the full,
grepped surface (`git grep -- "--template hierarchy|--template coding"`):

- **Built-in registry:** `renderTemplate`'s map shrinks to `{default}`; `--template` flag help + the
  unknown-name error list only `default`.
- **`demo/` (the BLOCKER — it rides `go test ./...` = `make check`):** `demo/coding-flow.sh:77` calls
  `mtt init --template coding`; `demo/coding_flow_test.go` runs that script. Migrate the script to **write
  `coding.yaml` into its work dir** (a heredoc, or copy from the repo `templates/coding.yaml` via a
  `$REPO_ROOT`-relative path the test passes in) then `mtt init --template ./coding.yaml`. Update
  `demo/README.md` + `demo/README.ru.md` (the `mtt init --template coding` prose) and `demo/doc.go` if it names it.
- **11 testscripts** use `--template hierarchy`/`coding`: `add_depends_on.txt`, `add_show.txt`, `batch.txt`,
  `dep.txt`, `init.txt`, `list_edit.txt`, `ready.txt`, `roadmap.txt`, `rm.txt`, `tags.txt`, `tree.txt`
  (several genuinely need epic/subtask types). Mechanism: a shared **`testscript` `Setup` hook**
  (`script_test.go`) writes `hierarchy.yaml` (and `coding.yaml` where used) into each script's `$WORK`; the
  scripts switch `--template hierarchy` → `--template $WORK/hierarchy.yaml` (a mechanical one-token edit that
  also exercises the new **file-path** source). `init.txt` additionally updates its built-in-list + unknown
  assertions and adds the new external-source cases.
- **`init_test.go:31`** (`--template coding`) migrates the same way (file-path fixture) or switches to `default`.
- **Docs (EN+RU) naming the removed built-ins as runnable commands:** `README.md`/`README.ru.md` (×2 each),
  `DESIGN.md`/`DESIGN.ru.md`, `FLOW_GUIDE.md`/`FLOW_GUIDE.ru.md` — updated in the Docs sweep (below), so no
  doc tells a user to run a now-removed built-in.

## Testing (TDD)

- **Classification** unit table: `default`→builtin; `x.yaml`/`./x`/`a/b` (and Windows `a\b`)→file;
  `https://…`→url; `http://…`→error; unknown bare→sorted-names error; scheme-less `host.com/x.yaml`→file with
  the `https://` hint on not-found.
- **`checkDecoded` / `ValidateTemplateBytes` (shared validation)**: rejects a bad prefix (empty / duplicate /
  **shell-metachar** — the injection case), `defaults != 1`, a bad `command_timeout`, and a topology/flow
  error — each writes **nothing**. **`Load` overlay regression**: existing `Load` tests stay green (two-file
  overlay + `Author` from `config.local` + tighten-only `require` intact after routing checks through
  `checkDecoded`).
- **Verbatim**: an external template containing `{{.ID}}` survives byte-for-byte (not eaten / not an error);
  `--name` with an external source is ignored + a stderr note is printed.
- **Fetch (fake `http.RoundTripper` into a *real* client, no socket)**: success; https-reject (`http://`);
  **redirect-to-http refuse** (fake transport serves a `302`→http target; the real client's `CheckRedirect` =
  `httpsOnlyRedirect` rejects it) **+** `httpsOnlyRedirect` as a pure-predicate unit test; HTTP 404/500 →
  `HTTP <status>` error; over-cap body → error (not silent truncation).
- **`confirmRemote` (pure)**: `y`/`yes`→proceed; `n`/empty→abort (no write); `autoYes`→proceed without reading;
  `isTTY=false && !autoYes`→refuse.
- **Notice + `--json`**: an external write prints the ⚠ block on stderr and sets `source` (`file:…`/`url:…`);
  the built-in prints neither; `template` still carries the raw arg.
- **git-flow.yaml**: loads + validates; the **description-keyword** test (`push`/`jq`/`--no-run`; `gh` matched
  as a **token**, not a substring).
- **Built-in `default` unchanged**: existing golden/e2e green; `default.yaml` loads + validates.
- **e2e** (`init.txt`): built-in `default`; **file-path** install (+ verbatim `{{.ID}}` survival +
  validate-fail-closed on a broken file); **non-TTY URL refuse** (errors before any fetch — no server); plus
  the migrated hierarchy/coding scripts (via the `$WORK` fixture).

## Docs

Sweep all parallel occurrences (EN+RU) per the design-docs-parallel-occurrences lesson:

- **CLI_REFERENCE (EN+RU):** `mtt init --template <name|path|url>` — the three forms, the resolution rule, the
  https-only + confirm + `--yes` + caps for URL, validate-before-write, the review notice, the `--json`
  `source` field; the built-in list is now just `default`.
- **README (EN+RU):** the `--template coding`/`hierarchy` mentions (×2 each) update — either to `default` or to
  the new `--template <url>` example; no README tells a user to run a removed built-in.
- **FLOW_GUIDE (EN+RU):** the forward-link resolves — "install a ready flow: `mtt init --template <url>`",
  pointing at `templates/git-flow.yaml`; any `--template hierarchy/coding` mention updated.
- **DESIGN (EN+RU):** a Shipped(t62) note — the minimal-built-in + bring-your-own strategy, the top-level
  `templates/` dir, the `checkDecoded` shared-validation refactor, the no-silent-traps / SEC2 posture, t24→t66
  unblocking the flagship; any `--template hierarchy/coding` mention updated.
- **`demo/README.md` + `demo/README.ru.md`** (+ `doc.go` if it names it): the `mtt init --template coding`
  prose reflects the migrated demo (writes `coding.yaml` then `--template ./coding.yaml`).
- **CLAUDE.md:** `internal/cli` (init external-source flow + confirm/notice), `internal/adapter/yaml`
  (classification/fetch/`checkDecoded`+`ValidateTemplateBytes`/verbatim; `Load` keeps its overlay), and the
  new root `templates` package.
- **CHANGELOG:** one `[Unreleased]` entry. **`templates/README.md`:** adaptable-samples framing + fetch how-to.

## Open seams (recorded, not built)

- **`--template <url>` for private/authed sources** — anonymous https only here; tokens/auth are a later ask.
- **Template index / discovery** — no registry; a direct file fetch only.
- **URL happy-path in `testscript`** — covered by unit tests with a fake transport (no socket, §5); a
  harness-served loopback URL for e2e is possible (start a server in `TestMain`, pass via env) but deferred
  unless the unit coverage proves thin (and it would reintroduce a real socket, against no-network-in-tests).
- **A checksum/pin for a fetched template** — out of scope (also a Non-goal); the loud notice +
  validate-before-write is the trust boundary (config-as-code is reviewed like a Makefile, SEC2).
