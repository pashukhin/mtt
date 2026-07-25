# t62 — Runnable init templates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `mtt init --template <arg>` accept a built-in name (`default` only), a local file path, or an https URL — validating external config-as-code before write, confirming remote fetches, and shipping the git-integration flagship + the de-embedded examples in a repo-root `templates/` directory.

**Architecture:** A pure classifier splits `<arg>` into builtin/file/url. Built-ins render from a new repo-root `templates` Go package (`//go:embed default.yaml`). External bytes are validated via a shared adapter check (`checkDecoded` = the exact provider checks `Load` already runs, + `Config.Validate` on the external path only) then written **verbatim**. URL fetch sits behind an injectable `http.RoundTripper` (real client, so `CheckRedirect` runs) with an interactive confirm. The CLI stays thin (classify → confirm → fetch → validate → write → notice). Follows the approved spec ([t62-init-templates.md](../specs/t62-init-templates.md), rev5).

**Tech Stack:** Go 1.23, cobra, testscript (txtar e2e), `gopkg.in/yaml.v3`, stdlib `net/http`.

## Global Constraints

- **TDD**: red → green → refactor; `make check` green before every commit; **each task leaves the tree green** (the file/url sources land before de-embedding coding/hierarchy, so no test is ever left broken).
- The spec is the decision record; on conflict, the spec wins. Do not relitigate the strategy (Option 3), built-in scope (`default` only), or the top-level `templates/` dir.
- **No new dependency**; TTY detection via stdlib `os.Stdin.Stat()` + `os.ModeCharDevice` (not `golang.org/x/term`). Keep `go.mod` at `go 1.23.1`.
- **No network in tests** — URL paths unit-tested with a fake `http.RoundTripper` (no socket); e2e covers built-in + file-path + non-TTY-url-refuse only.
- **Verbatim external**: external templates are written byte-for-byte (no `{{.Name}}`); only the built-in `default` is `{{.Name}}`-templated.
- **checkDecoded ⊄ Config.Validate**: `Load` must stay byte-identical — `Config.Validate` lives in the external `ValidateTemplateBytes` path only (init owns the domain check there), never in `Load`.
- **Config is code (SEC2)**: the `.mtt/config.yaml` wording change (Task 7) is reviewed like a Makefile; `TestRepoDogfoodConfig` stays green (updated red-first).
- English docs are source of truth; every DESIGN/CLI_REFERENCE/README/FLOW_GUIDE change lands in the RU mirror in the same task.
- Commit trailer: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`.

---

### Task 1: Adapter validation refactor — `checkDecoded` + `ValidateTemplateBytes`

**Files:**
- Modify: `internal/adapter/yaml/load.go` (extract `checkDecoded`; route `Load` through it; add `ValidateTemplateBytes`)
- Test: `internal/adapter/yaml/load_test.go` (or a new `validate_test.go`)

**Interfaces:**
- Produces: `func checkDecoded(yc ymlConfig) (mtt.Config, map[string]string, time.Duration, error)` (package-private) and `func ValidateTemplateBytes(data []byte) error` (exported). Consumed by Tasks 3–4.

- [ ] **Step 1: Write the failing test** (`internal/adapter/yaml/validate_test.go`):

```go
package yaml

import (
	"strings"
	"testing"
)

const goodTemplate = `version: 1
project: {name: x}
types:
  - name: task
    prefix: t
    default: true
    statuses:
      - {name: tbd, kind: initial}
      - {name: done, kind: terminal}
    transitions:
      - {from: tbd, to: done}
`

func TestValidateTemplateBytes(t *testing.T) {
	if err := ValidateTemplateBytes([]byte(goodTemplate)); err != nil {
		t.Fatalf("good template: %v", err)
	}
	// bad prefix (shell metachar) — the injection case checkPrefixes must reject
	bad := strings.Replace(goodTemplate, "prefix: t", "prefix: \"t;rm\"", 1)
	if err := ValidateTemplateBytes([]byte(bad)); err == nil {
		t.Fatal("shell-metachar prefix must be rejected")
	}
	// domain-invalid but provider-valid: a transition to a non-existent status
	dom := strings.Replace(goodTemplate, "to: done", "to: ghost", 1)
	if err := ValidateTemplateBytes([]byte(dom)); err == nil {
		t.Fatal("domain-invalid template must be rejected by Config.Validate")
	}
	// not YAML at all
	if err := ValidateTemplateBytes([]byte("::not yaml::")); err == nil {
		t.Fatal("unparseable template must error")
	}
}
```

- [ ] **Step 2: Run to verify it fails** — `go test ./internal/adapter/yaml/ -run TestValidateTemplateBytes -v` → FAIL (`ValidateTemplateBytes` undefined).

- [ ] **Step 3: Implement** — in `load.go`, extract the post-overlay checks and add the external validator. Replace the inline `toDomain`/`checkPrefixes`/`parseCommandTimeout` block in `Load` with a call to `checkDecoded`:

```go
// checkDecoded runs the YAML provider's post-decode checks — exactly what Load
// applies after its overlay: toDomain + checkPrefixes (single default, prefix
// present/unique/letters-only — the shell-safety boundary) + parseCommandTimeout.
// It deliberately does NOT run Config.Validate (that is the caller's, per Load's
// contract); the external-template path adds it in ValidateTemplateBytes.
func checkDecoded(yc ymlConfig) (mtt.Config, map[string]string, time.Duration, error) {
	cfg, prefixes := yc.toDomain()
	if err := checkPrefixes(cfg, prefixes); err != nil {
		return mtt.Config{}, nil, 0, err
	}
	timeout, err := parseCommandTimeout(yc.CommandTimeout)
	if err != nil {
		return mtt.Config{}, nil, 0, err
	}
	return cfg, prefixes, timeout, nil
}

// ValidateTemplateBytes fail-closed-validates an external (path/url) template:
// decode a single doc → the provider checks (checkDecoded) → Config.Validate.
// Init legitimately IS the caller that owns the domain check for an untrusted
// template, so this is the one place Config.Validate rides the load path.
func ValidateTemplateBytes(data []byte) error {
	var yc ymlConfig
	if err := goyaml.Unmarshal(data, &yc); err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	cfg, _, _, err := checkDecoded(yc)
	if err != nil {
		return err
	}
	return cfg.Validate()
}
```

And in `Load`, after the two `decodeInto` calls, replace the three inline steps with:

```go
	cfg, prefixes, timeout, err := checkDecoded(yc)
	if err != nil {
		return mtt.Config{}, Settings{}, err
	}
```

(`Load` keeps `committedRequire` capture, the `require` OR-combine, and `Author: yc.Author` — unchanged. The `goyaml` import alias already exists in the package; if not, it is `goyaml "gopkg.in/yaml.v3"`.)

- [ ] **Step 4: Add the Load-unchanged regression test** (same file):

```go
func TestLoadDoesNotRunConfigValidate(t *testing.T) {
	// A provider-valid but DOMAIN-invalid config still LOADS (Load's contract:
	// domain invariants are the caller's). ValidateTemplateBytes rejects it (above),
	// but Load must not — else read commands regress.
	dir := t.TempDir()
	mustWrite(t, dir, strings.Replace(goodTemplate, "to: done", "to: ghost", 1))
	if _, _, err := Load(dir); err != nil {
		t.Fatalf("Load must not run Config.Validate (domain-invalid still loads): %v", err)
	}
}
```

Add a `mustWrite(t, dir, content)` helper that writes `<dir>/.mtt/config.yaml` (MkdirAll `.mtt`), if the test file lacks one.

- [ ] **Step 5: Run** — `go test ./internal/adapter/yaml/ -run 'TestValidateTemplateBytes|TestLoadDoesNotRunConfigValidate|TestLoad' -v` → PASS; `make check` → green.

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/yaml/load.go internal/adapter/yaml/validate_test.go
git commit -m "t62: extract checkDecoded + ValidateTemplateBytes (Load byte-identical; Config.Validate external-only)"
```

---

### Task 2: `--template` classifier (pure function)

**Files:**
- Create: `internal/adapter/yaml/templatesource.go`
- Test: `internal/adapter/yaml/templatesource_test.go`

**Interfaces:**
- Produces: `type SourceKind int` (`SourceBuiltin`/`SourceFile`/`SourceURL`) and `func ClassifyTemplate(arg string) (SourceKind, string, error)`. Consumed by Task 3 (CLI).

- [ ] **Step 1: Write the failing test**:

```go
package yaml

import "testing"

func TestClassifyTemplate(t *testing.T) {
	cases := []struct {
		arg  string
		kind SourceKind
		err  bool
	}{
		{"default", SourceBuiltin, false},
		{"coding", SourceBuiltin, false}, // bare unknown name is still "builtin" (name-resolution errors later)
		{"x.yaml", SourceFile, false},
		{"./x.yml", SourceFile, false},
		{"a/b/c", SourceFile, false},
		{"https://h/x.yaml", SourceURL, false},
		{"http://h/x.yaml", 0, true}, // https only
	}
	for _, c := range cases {
		k, _, err := ClassifyTemplate(c.arg)
		if (err != nil) != c.err {
			t.Fatalf("%q: err=%v want err=%v", c.arg, err, c.err)
		}
		if err == nil && k != c.kind {
			t.Fatalf("%q: kind=%d want %d", c.arg, k, c.kind)
		}
	}
}
```

- [ ] **Step 2: Run** → FAIL (`ClassifyTemplate` undefined).

- [ ] **Step 3: Implement** (`templatesource.go`):

```go
package yaml

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SourceKind is how a `mtt init --template <arg>` value resolves.
type SourceKind int

const (
	SourceBuiltin SourceKind = iota // a compiled-in template name (default)
	SourceFile                      // a local file path
	SourceURL                       // an https URL
)

// ClassifyTemplate resolves the source kind of a --template arg (pure): https://
// → URL (http:// is a hard error); a path shape (separator, or .yaml/.yml suffix)
// → file; otherwise a bare token → builtin name (name lookup errors later).
func ClassifyTemplate(arg string) (SourceKind, string, error) {
	switch {
	case strings.HasPrefix(arg, "https://"):
		return SourceURL, arg, nil
	case strings.HasPrefix(arg, "http://"):
		return 0, "", fmt.Errorf("remote templates must be https (got %q)", arg)
	case strings.ContainsRune(arg, '/') || strings.ContainsRune(arg, filepath.Separator) ||
		strings.HasSuffix(arg, ".yaml") || strings.HasSuffix(arg, ".yml"):
		return SourceFile, arg, nil
	default:
		return SourceBuiltin, arg, nil
	}
}
```

- [ ] **Step 4: Run** — `go test ./internal/adapter/yaml/ -run TestClassifyTemplate -v` → PASS; `make check` → green.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/yaml/templatesource.go internal/adapter/yaml/templatesource_test.go
git commit -m "t62: ClassifyTemplate — resolve --template into builtin/file/url"
```

---

### Task 3: File-path source end-to-end (verbatim install + validate + notice + `--json source` + `--name` note)

**Files:**
- Modify: `internal/adapter/yaml/init.go` (extract `writeConfig`; add `InstallConfig(root, data, force)`)
- Modify: `internal/cli/init.go` (classify → file path: read+install+notice; `--json` `source`; `--name` note)
- Modify: `internal/cli/json.go` (`initJSON` gains `Source`)
- Test: `internal/cli/testdata/scripts/init.txt` (append fresh-project file-path cases)

**Interfaces:**
- Consumes: `ValidateTemplateBytes` (T1), `ClassifyTemplate` (T2).
- Produces: `func InstallConfig(root string, data []byte, force bool) error` (validate-then-write-verbatim). Consumed by Task 4 (url).

- [ ] **Step 1: Write the failing e2e.** **txtar ordering matters:** `init.txt` currently ends with a single file section `-- empty/.keep --` (line 63). Insert the **command** block **immediately before** `-- empty/.keep --` (all commands must precede all file blocks), and add the two new file sections `-- ext.yaml --`/`-- bad.yaml --` **after** `-- empty/.keep --`. Command block:

```
# --- file-path source: verbatim install of an external template ---
cd $WORK
mkdir fileproj
cd fileproj
exec mtt init --template $WORK/ext.yaml
stdout 'initialized'
stderr 'review .mtt/config.yaml before your first'
# verbatim: the external {{.ID}} runtime placeholder survives byte-for-byte
grep '\{\{\.ID\}\}' .mtt/config.yaml
# --json carries the source provenance
exec mtt init --template $WORK/ext.yaml --force --json
stdout '"source": "file:'
# --name is ignored for an external source (with a stderr note, never silent)
exec mtt init --template $WORK/ext.yaml --force --name ignored
stderr 'note: --name is ignored'
# a broken external template fails closed — nothing written
cd $WORK
mkdir badproj
cd badproj
! exec mtt init --template $WORK/bad.yaml
! exists .mtt/config.yaml
# a scheme-less URL-looking arg (dotted host + / + .yaml) misses as a file, with an https hint
! exec mtt init --template raw.githubusercontent.com/x/y.yaml
stderr 'did you mean https://'
```

File sections (after `-- empty/.keep --`):

```
-- ext.yaml --
version: 1
project: {name: ext}
command_timeout: 5m
types:
  - name: task
    prefix: t
    default: true
    statuses:
      - {name: tbd, kind: initial}
      - {name: done, kind: terminal}
    transitions:
      - {from: tbd, to: done, description: "close {{.ID}}"}
-- bad.yaml --
version: 1
project: {name: bad}
types:
  - name: task
    prefix: "t;rm"
    default: true
    statuses: [{name: tbd, kind: initial}]
    transitions: []
```

- [ ] **Step 2: Run** → FAIL (`--template $WORK/ext.yaml` today hits `renderTemplate` → unknown-template error).

- [ ] **Step 3: Implement — adapter.** In `init.go`, extract the write half of `Init` into `writeConfig`, and add `InstallConfig`:

```go
// writeConfig writes content to <root>/.mtt/config.yaml atomically, dropping the
// .gitignore and refusing overwrite without force (extracted from Init so both
// the builtin render path and the external verbatim path share it).
func writeConfig(root string, content []byte, force bool) error {
	dir := filepath.Join(root, dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	if err := writeGitignore(dir); err != nil {
		return err
	}
	dst := filepath.Join(dir, configName)
	if !force {
		if _, statErr := os.Stat(dst); statErr == nil {
			return ErrAlreadyInitialized
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("stat %s: %w", dst, statErr)
		}
	}
	return atomicWrite(dst, content)
}

// InstallConfig validates external template bytes (fail-closed) then writes them
// verbatim. Used for --template <path|url>; the built-in path stays Init.
func InstallConfig(root string, data []byte, force bool) error {
	if err := ValidateTemplateBytes(data); err != nil {
		return err
	}
	return writeConfig(root, data, force)
}
```

Refactor `Init` to call `writeConfig`:

```go
func Init(root, tmplName, projectName string, force bool) error {
	content, err := renderTemplate(tmplName, projectName)
	if err != nil {
		return err
	}
	return writeConfig(root, content, force)
}
```

- [ ] **Step 4: Implement — CLI.** In `internal/cli/init.go`, classify and branch; add the `source` to `initJSON`. Body of `RunE` becomes:

```go
kind, value, err := yaml.ClassifyTemplate(tmpl)
if err != nil {
	return err
}
var source string
switch kind {
case yaml.SourceBuiltin:
	if err := yaml.Init(base, value, projectName, force); err != nil {
		return err
	}
	source = "builtin:" + value
case yaml.SourceFile:
	data, rerr := os.ReadFile(value)
	if rerr != nil {
		return templateReadError(value, rerr) // adds the scheme-less-URL hint
	}
	if name != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "note: --name is ignored for an external template (%s)\n", value)
	}
	if err := yaml.InstallConfig(base, data, force); err != nil {
		return err
	}
	source = "file:" + value
	printExternalNotice(cmd, value)
case yaml.SourceURL:
	return errURLNotYetWired // replaced in Task 4
}
```

Add helpers in `init.go`:

```go
// templateReadError wraps a failed --template file read, hinting at https:// when
// the arg looks like a scheme-less URL (a dotted host segment before the first /,
// but not a ./ or ../ relative path).
func templateReadError(path string, err error) error {
	if i := strings.IndexByte(path, '/'); i > 0 {
		head := path[:i]
		if head != "." && head != ".." && strings.ContainsRune(head, '.') {
			return fmt.Errorf("read template %q: %w (did you mean https://%s ?)", path, err, path)
		}
	}
	return fmt.Errorf("read template %q: %w", path, err)
}

// printExternalNotice is the loud SEC2 review notice for an external install.
func printExternalNotice(cmd *cobra.Command, source string) {
	fmt.Fprintf(cmd.ErrOrStderr(),
		"\n⚠  installed .mtt/config.yaml from %s\n"+
			"   this config's gate/post commands run via `sh -c` on your transitions.\n"+
			"   review .mtt/config.yaml before your first `mtt` move.\n", source)
}
```

Set `initJSON.Source` where the JSON is emitted (`Source: source`), and keep the existing `Template: tmpl`. Add `Source string \`json:"source,omitempty"\`` to `initJSON` in `json.go`. The human success line stays `initialized .mtt/config.yaml (template %q)` with `tmpl`. `errURLNotYetWired` is a temporary `errors.New("url templates land in the next task")` removed in Task 4 (its e2e is added there).

- [ ] **Step 5: Run** — `go test ./internal/cli/ -run 'TestScript/init' -v` → PASS; `go test ./internal/adapter/yaml/ -run TestInit` → PASS; `make check` → green.

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/yaml/init.go internal/cli/init.go internal/cli/json.go internal/cli/testdata/scripts/init.txt
git commit -m "t62: --template <path> — verbatim install of an external template (validate-before-write, notice, --json source)"
```

---

### Task 4: URL source (RoundTripper fetcher + confirm + `--yes` + non-TTY refuse)

**Files:**
- Create: `internal/adapter/yaml/templatefetch.go` (the `Fetcher` + `httpsOnlyRedirect`)
- Create: `internal/cli/confirm.go` (`confirmRemote`)
- Modify: `internal/cli/init.go` (wire the url branch + `--yes` flag + TTY detection)
- Test: `internal/adapter/yaml/templatefetch_test.go`, `internal/cli/confirm_test.go`, `internal/cli/testdata/scripts/init.txt` (non-TTY refuse)

**Interfaces:**
- Consumes: `InstallConfig` (T3).
- Produces: `func NewFetcher() *Fetcher`, `func (*Fetcher) Fetch(url string) ([]byte, error)`, `func httpsOnlyRedirect(req *http.Request, via []*http.Request) error`; `func confirmRemote(in io.Reader, isTTY, autoYes bool) (bool, error)`.

- [ ] **Step 1: Write the failing unit tests.** `templatefetch_test.go`:

```go
package yaml

import (
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func resp(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestFetchSuccess(t *testing.T) {
	f := newFetcherWithTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return resp(200, "version: 1\n"), nil
	}))
	b, err := f.Fetch("https://h/x.yaml")
	if err != nil || string(b) != "version: 1\n" {
		t.Fatalf("fetch: %q %v", b, err)
	}
}

func TestFetchNon200(t *testing.T) {
	f := newFetcherWithTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return resp(404, "nope"), nil
	}))
	if _, err := f.Fetch("https://h/x.yaml"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("want HTTP 404 error, got %v", err)
	}
}

func TestFetchOverCap(t *testing.T) {
	big := strings.Repeat("a", (1<<20)+10) // > 1 MiB
	f := newFetcherWithTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return resp(200, big), nil
	}))
	if _, err := f.Fetch("https://h/x.yaml"); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("want too-large error, got %v", err)
	}
}

func TestHTTPSOnlyRedirect(t *testing.T) {
	httpReq, _ := http.NewRequest("GET", "http://evil/x", nil)
	if err := httpsOnlyRedirect(httpReq, nil); err == nil {
		t.Fatal("redirect to http must be refused")
	}
	httpsReq, _ := http.NewRequest("GET", "https://ok/x", nil)
	if err := httpsOnlyRedirect(httpsReq, nil); err != nil {
		t.Fatalf("https redirect must be allowed: %v", err)
	}
}

// The seam-wiring test the spec §5 justified: a 302 → http hop must be refused by
// the REAL client's CheckRedirect (a fake transport alone can't prove this — only
// a real http.Client runs CheckRedirect). The transport serves the 302; the
// client's httpsOnlyRedirect must reject the http Location before a second hop.
func TestFetchRefusesRedirectToHTTP(t *testing.T) {
	f := newFetcherWithTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		h := make(http.Header)
		h.Set("Location", "http://evil/x.yaml")
		return &http.Response{StatusCode: 302, Body: io.NopCloser(strings.NewReader("")), Header: h}, nil
	}))
	if _, err := f.Fetch("https://h/x.yaml"); err == nil {
		t.Fatal("a redirect to http must be refused by the real client's CheckRedirect")
	}
}
```

(Add `"io"` to imports.) `confirm_test.go`:

```go
package cli

import (
	"strings"
	"testing"
)

func TestConfirmRemote(t *testing.T) {
	yes, _ := confirmRemote(strings.NewReader("y\n"), true, false)
	if !yes {
		t.Fatal("y must proceed")
	}
	no, _ := confirmRemote(strings.NewReader("\n"), true, false)
	if no {
		t.Fatal("empty must abort")
	}
	auto, _ := confirmRemote(strings.NewReader(""), false, true) // --yes, no read
	if !auto {
		t.Fatal("--yes must proceed without reading")
	}
	refuse, err := confirmRemote(strings.NewReader(""), false, false) // non-TTY, no --yes
	if refuse || err == nil {
		t.Fatal("non-TTY without --yes must refuse with an error")
	}
}
```

- [ ] **Step 2: Run** → FAIL (undefined `newFetcherWithTransport`/`confirmRemote`).

- [ ] **Step 3: Implement the fetcher** (`templatefetch.go`):

```go
package yaml

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	fetchTimeout = 30 * time.Second
	fetchCap     = 1 << 20 // 1 MiB
)

// httpsOnlyRedirect refuses a redirect whose next hop is not https (the real
// http.Client redirect loop invokes this — unit-testable as a pure predicate).
func httpsOnlyRedirect(req *http.Request, _ []*http.Request) error {
	if req.URL.Scheme != "https" {
		return fmt.Errorf("refusing redirect to non-https %q", req.URL)
	}
	return nil
}

// Fetcher downloads a template over https. The transport is injectable so tests
// run without a socket (a fake RoundTripper), while the real client's redirect
// loop still enforces httpsOnlyRedirect.
type Fetcher struct{ client *http.Client }

// NewFetcher returns a Fetcher over a real https client (30s timeout, https-only
// redirects).
func NewFetcher() *Fetcher {
	return &Fetcher{client: &http.Client{Timeout: fetchTimeout, CheckRedirect: httpsOnlyRedirect}}
}

func newFetcherWithTransport(rt http.RoundTripper) *Fetcher {
	return &Fetcher{client: &http.Client{Timeout: fetchTimeout, CheckRedirect: httpsOnlyRedirect, Transport: rt}}
}

// Fetch GETs url (https only), enforces a non-200 error and the 1 MiB size cap
// (LimitReader(cap+1) so an over-cap body errors instead of silently truncating).
func (f *Fetcher) Fetch(url string) ([]byte, error) {
	resp, err := f.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: HTTP %d", url, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, fetchCap+1))
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	if len(data) > fetchCap {
		return nil, fmt.Errorf("fetch %s: template too large (> %d bytes)", url, fetchCap)
	}
	return data, nil
}
```

- [ ] **Step 4: Implement confirm** (`internal/cli/confirm.go`):

```go
package cli

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"
)

// confirmRemote decides whether to fetch a remote template. --yes proceeds
// without reading. A non-TTY without --yes refuses (no silent remote fetch).
// Otherwise it reads one line and proceeds only on y/yes.
func confirmRemote(in io.Reader, isTTY, autoYes bool) (bool, error) {
	if autoYes {
		return true, nil
	}
	if !isTTY {
		return false, errors.New("refusing to fetch a remote template without confirmation; pass --yes")
	}
	line, _ := bufio.NewReader(in).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

// stdinIsTTY reports whether stdin is a terminal (stdlib; no x/term dep).
func stdinIsTTY() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
```

- [ ] **Step 5: Wire the url branch** in `init.go` — replace `return errURLNotYetWired` with:

```go
case yaml.SourceURL:
	fmt.Fprintf(cmd.ErrOrStderr(), "install template from %s? [y/N] ", value)
	ok, cerr := confirmRemote(cmd.InOrStdin(), stdinIsTTY(), autoYes)
	if cerr != nil {
		return cerr
	}
	if !ok {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "aborted")
		return err
	}
	data, ferr := yaml.NewFetcher().Fetch(value)
	if ferr != nil {
		return ferr
	}
	if name != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "note: --name is ignored for an external template (%s)\n", value)
	}
	if err := yaml.InstallConfig(base, data, force); err != nil {
		return err
	}
	source = "url:" + value
	printExternalNotice(cmd, value)
```

Add the flag: `cmd.Flags().BoolVar(&autoYes, "yes", false, "skip the confirmation prompt for a remote --template URL")` and declare `var autoYes bool`. Delete `errURLNotYetWired`.

- [ ] **Step 6: e2e non-TTY refuse** — add this block to `init.txt` **before `-- empty/.keep --`** (with all the other command blocks — every command precedes every `-- file --` section). testscript stdin is a non-TTY pipe, so this errors before any fetch — no server needed:

```
# --- a URL template is refused without --yes when stdin is not a TTY ---
cd $WORK
mkdir urlproj
cd urlproj
! exec mtt init --template https://example.com/x.yaml
stderr 'refusing to fetch a remote template without confirmation'
! exists .mtt/config.yaml
```

- [ ] **Step 7: Run** — `go test ./internal/adapter/yaml/ ./internal/cli/ -run 'TestFetch|TestHTTPSOnlyRedirect|TestConfirmRemote|TestScript/init' -v` → PASS; `make check` → green.

- [ ] **Step 8: Commit**

```bash
git add internal/adapter/yaml/templatefetch.go internal/cli/confirm.go internal/cli/init.go internal/cli/testdata/scripts/init.txt internal/adapter/yaml/templatefetch_test.go internal/cli/confirm_test.go
git commit -m "t62: --template <url> — https fetcher (RoundTripper seam) + confirm/--yes/non-TTY refuse"
```

---

### Task 5: Top-level `templates/` package (embed `default` only) + de-embed coding/hierarchy + migrate all consumers

**This task is atomic** — de-embedding the built-ins and migrating every `--template coding|hierarchy` consumer must land together, or `make check` goes red.

> **Migration surface is large and grep-invisible.** A `--template hierarchy` *string* grep (spec §7) misses every Go test that passes the flag as **separate args** (`runRoot(t, "init", "--template", "hierarchy")`) or calls the adapter directly (`Init(root, "hierarchy", …)`). The full surface, enumerated below, is ~9 Go test files across two packages **plus** the 11 testscripts + demo. Missing any one reds `make check`.

**Files:**
- Create: `templates/templates.go` (package `templates`, `//go:embed default.yaml`), `templates/default.yaml` (moved, keeps `{{.Name}}`), `templates/coding.yaml` + `templates/hierarchy.yaml` (moved **and de-templated** — literal name), `templates/git-flow.yaml` (valid placeholder now, authored Task 7), `templates/README.md`
- Delete (via `git mv`): `internal/adapter/yaml/templates/{default,coding,hierarchy}.yaml`; remove the `//go:embed` in `internal/adapter/yaml/templates.go`; remove now-unused goldens `internal/adapter/yaml/testdata/golden/{coding,hierarchy}.yaml`
- Modify (impl): `internal/adapter/yaml/templates.go` (source `default` from the package; registry → `{default}`), `internal/cli/init.go` (flag help)
- Modify (**adapter Go tests** — the undercounted surface): `internal/adapter/yaml/init_test.go` (`TestRenderGolden`→`{default}`; `TestRenderUnknownTemplate`→valid list is `{default}`; `TestInitKeepsExistingGitignore`+`TestInit` use `"default"`), `internal/adapter/yaml/load_test.go` (`TestInitLoadValidate`→`{default}`), `internal/adapter/yaml/task_test.go` (`initHierarchy` helper → file-path via `InstallConfig`; shared by task_test/task_security_test/perm_unix_test — ~20 tests)
- Modify (**cli Go tests**): `internal/cli/init_test.go:31` (`coding`→`default` or file-path), and the 8 `runRoot/runOut(…, "--template", "hierarchy")` sites (`add_test.go:22`, `edit_test.go:11/42/58`, `list_test.go:19`, `project_test.go:16`, `show_json_test.go`, `show_test.go`) → a shared `hierarchyTemplate(t)` path helper
- Modify (e2e + demo): `internal/cli/script_test.go` (shared `Setup` + `repoRoot`), the **11** testscripts, `demo/coding-flow.sh`, `demo/coding_flow_test.go`

**Interfaces:**
- Produces: `templates.Default() []byte`, `templates.BuiltinNames() []string` (root package). Consumed by `internal/adapter/yaml/templates.go`. Test helpers: `repoRoot(t)` (go.mod-anchored, one per test package that needs it) and `hierarchyTemplate(t)` (cli).

- [ ] **Step 1: Move + de-template + create the package.** `git mv internal/adapter/yaml/templates/default.yaml templates/default.yaml` (same for coding, hierarchy). **Critical de-template:** all three files carry unquoted `name: {{.Name}}` on line 3, which is **invalid standalone YAML** (`yaml.v3` parses `{{.Name}}` as a flow map → "cannot unmarshal !!map into string"). `default.yaml` stays as-is (it is the **built-in**, rendered through `text/template` — `{{.Name}}` is substituted). But `coding.yaml` and `hierarchy.yaml` now travel the **verbatim file-path** source, so replace their `name: {{.Name}}` with a literal `name: my-project`. (Verify: `go run` a quick `yaml.Unmarshal` or just rely on Step 6's tests — a non-de-templated file fails `ValidateTemplateBytes` at install.) Create `templates/templates.go`:

```go
// Package templates holds mtt's init config templates as plain files so they are
// URL-fetchable (raw.githubusercontent.com/…/templates/<name>.yaml). Only the
// minimal built-in `default` is embedded in the binary; the rest are hosted
// examples (coding, hierarchy, git-flow) — de-embedded per the t62 minimal-
// built-in strategy.
package templates

import _ "embed"

//go:embed default.yaml
var defaultYAML []byte

// Default returns the embedded default template bytes (the sole built-in).
func Default() []byte { return defaultYAML }

// BuiltinNames returns the compiled-in template names (only "default").
func BuiltinNames() []string { return []string{"default"} }
```

Create `templates/README.md` (adaptable-samples framing):

```markdown
# mtt config templates

Adaptable **samples**, not mandates — the engine is the product; a flow is
per-project data. Only `default.yaml` is built into the binary (`mtt init`).
The rest are examples you install with a path or URL:

    mtt init --template ./templates/git-flow.yaml
    mtt init --template https://raw.githubusercontent.com/pashukhin/mtt/main/templates/git-flow.yaml

- `default.yaml`   — minimal flat flow (task + chore).
- `coding.yaml`    — a dev flow (feature / bugfix / refactor).
- `hierarchy.yaml` — epics + tasks + subtasks.
- `git-flow.yaml`  — the git-integration flagship (branch → PR → deliver). Review
  its gate/post commands before use; it assumes a pushable `main`, `gh`+`jq`, and
  the repo's squash-merge title = PR title.
```

Create a **valid** placeholder `templates/git-flow.yaml` for now — a copy of the **de-templated** `templates/coding.yaml` (literal `name:`, so it parses standalone; a copy of `default.yaml` would carry `{{.Name}}` and be invalid). Nothing validates `git-flow.yaml` until Task 7, but keeping it standalone-valid avoids a surprise if a test lands early.

- [ ] **Step 2: Rewire `renderTemplate`** (`internal/adapter/yaml/templates.go`) — drop the `//go:embed`, source `default` from the package, shrink the registry:

```go
package yaml

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	roottemplates "github.com/pashukhin/mtt/templates"
)

// builtin returns the embedded bytes for a built-in name, or false.
func builtin(name string) ([]byte, bool) {
	if name == "default" {
		return roottemplates.Default(), true
	}
	return nil, false
}

func templateNames() []string { return roottemplates.BuiltinNames() }

// renderTemplate renders the named BUILT-IN template, substituting the project
// name. External templates never reach here (they are written verbatim).
func renderTemplate(name, projectName string) ([]byte, error) {
	raw, ok := builtin(name)
	if !ok {
		return nil, fmt.Errorf("unknown template %q (valid: %s)", name, strings.Join(templateNames(), ", "))
	}
	tmpl, err := template.New(name).Option("missingkey=error").Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("parse template %q: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct{ Name string }{Name: projectName}); err != nil {
		return nil, fmt.Errorf("render template %q: %w", name, err)
	}
	return buf.Bytes(), nil
}
```

Update the `--template` flag help in `internal/cli/init.go`: `"starter template: default | a file path | an https URL"`.

- [ ] **Step 3: Migrate the adapter Go tests** (`internal/adapter/yaml`). These call the built-in names directly and would all red on de-embed:
  - `init_test.go` `TestRenderGolden` (line 16): change the loop to `[]string{"default"}` (coding/hierarchy no longer render through `renderTemplate`).
  - `init_test.go` `TestRenderUnknownTemplate` (line 47): the valid-names list is now `{default}` — change the asserted loop to `[]string{"default"}` (and optionally assert the error does NOT list `coding`/`hierarchy`).
  - `init_test.go` `TestInitKeepsExistingGitignore` (line 99) + `TestInit` (line 127): `Init(root, "default", "demo", …)` instead of `"coding"`.
  - `load_test.go` `TestInitLoadValidate` (line 12): loop → `[]string{"default"}` (the hosted examples are covered by Task 7's `TestExamplesLoadAndValidate`).
  - `task_test.go` `initHierarchy` (line 168) — the shared helper (~20 callers in task_test/task_security_test/perm_unix_test) now installs the **hosted** hierarchy example via the file-path source:

```go
func initHierarchy(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	data, err := os.ReadFile(filepath.Join(repoRoot(), "templates", "hierarchy.yaml"))
	if err != nil {
		t.Fatalf("read hierarchy example: %v", err)
	}
	if err := InstallConfig(root, data, false); err != nil {
		t.Fatalf("install hierarchy: %v", err)
	}
	return root
}
```

  Add a package-test **no-arg `repoRoot() string`** helper anchored via `runtime.Caller(0)` (the test file's own dir) walked up to the dir containing `go.mod` — robust regardless of cwd, and usable from the testscript `Setup` (which has no `*testing.T`). `git rm` the now-unused `testdata/golden/{coding,hierarchy}.yaml`.

- [ ] **Step 4: Migrate the cli Go tests** (`internal/cli`). Add a shared helper + a `repoRoot(t)` (go.mod-anchored) in a cli test file:

```go
// hierarchyTemplate returns the repo's hosted hierarchy example path — the
// file-path replacement for the removed `hierarchy` built-in name.
func hierarchyTemplate(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(), "templates", "hierarchy.yaml")
}
```

  (Add the same no-arg `repoRoot() string` helper — `runtime.Caller`-anchored — to a cli test file.) Then in each of `add_test.go:22`, `edit_test.go:11/42/58`, `list_test.go:19`, `project_test.go:16`, `show_json_test.go`, `show_test.go`, replace the literal `"hierarchy"` arg with `hierarchyTemplate(t)` (e.g. `runRoot(t, "init", "--template", hierarchyTemplate(t))`). In `init_test.go:31`, replace `"coding"` with `"default"` (the force-init test needs no coding-specific types).

- [ ] **Step 5: Shared testscript fixture Setup.** In `internal/cli/script_test.go`, add a `Setup` that writes the repo's `templates/hierarchy.yaml` + `templates/coding.yaml` into every script's `$WORK`, so migrated scripts reference `$WORK/hierarchy.yaml`. Pin the repo root via `go.mod` (mirroring `dogfood_test.go`):

```go
// in testscript.Params:
Setup: func(env *testscript.Env) error {
	for _, name := range []string{"hierarchy.yaml", "coding.yaml"} {
		data, err := os.ReadFile(filepath.Join(repoRoot(), "templates", name))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(env.WorkDir, name), data, 0o644); err != nil {
			return err
		}
	}
	return nil
},
```

(The `Setup` uses the same no-arg `repoRoot()` — `runtime.Caller`-anchored — added in Step 4/its cli test file.)

- [ ] **Step 6: Migrate the 11 testscripts.** In each of `add_depends_on.txt`, `add_show.txt`, `batch.txt`, `dep.txt`, `init.txt`, `list_edit.txt`, `ready.txt`, `roadmap.txt`, `rm.txt`, `tags.txt`, `tree.txt`, replace `--template hierarchy` → `--template $WORK/hierarchy.yaml` and `--template coding` → `--template $WORK/coding.yaml` (mechanical; the `$WORK` fixtures come from the Step 5 Setup). In `init.txt`, add (as a **command** block, before `-- empty/.keep --`): `! exec mtt init --template coding` / `stderr 'unknown template "coding"'` (coding is no longer a built-in name → classified as a bare builtin name → `renderTemplate` unknown-name error listing `default`).

- [ ] **Step 7: Migrate the demo.** In `demo/coding-flow.sh`, before the init, materialize the example: `cp "$REPO_ROOT/templates/coding.yaml" ./coding.yaml` then `"$MTT" init --template ./coding.yaml`. `demo/coding_flow_test.go` already computes `repoRoot` — export it into the script env (`cmd.Env = append(cmd.Env, "REPO_ROOT="+repoRoot)`). (`init_test.go:31` was already migrated in Step 4.)

- [ ] **Step 8: Run** — `go test ./... -run 'TestScript|TestCodingFlowDemo|TestInit|TestRender|TestInitLoadValidate|Task|Security|Perm' -v` → PASS; `go build ./...` clean (no dangling embed). `make check` → green.

- [ ] **Step 9: Commit** (the `git mv`s already staged the deletions of the old template files; only the goldens need an explicit `git rm`):

```bash
git rm internal/adapter/yaml/testdata/golden/coding.yaml internal/adapter/yaml/testdata/golden/hierarchy.yaml
git add templates/ internal/adapter/yaml/ internal/cli/ demo/
git commit -m "t62: templates/ dir (embed default only); de-embed coding/hierarchy; migrate all consumers (Go tests + testscripts + demo) to file-path"
```

---

### Task 6: Machine→human review ordering wording in `.mtt/config.yaml` (Goal 5/§8)

Reword the review-stage descriptions so "agent gate first, human sign-off strictly after" is **explicit**, in both the `task` and `chore` types. Pure wording — no status/edge added; the ordering is already structurally enforced. Doing this **before** Task 7 lets `git-flow.yaml` derive the corrected wording from the config.

**Files:**
- Modify: `internal/adapter/yaml/dogfood_test.go` (`TestRepoDogfoodConfig` — **add** red-first assertions), `.mtt/config.yaml`

**Note on the test:** `TestRepoDogfoodConfig` asserts every status/edge description is **non-empty** and pins **specific substrings** (`speccing`→"superpowers:brainstorming", `approved`→"gh pr create", `deliver`→"pull main"/"mtt note add"/"--no-run", chore `impl_review`→"it must be a"). It does **not** yet assert the review-ordering wording. So this task **adds** assertions (it does not "change pinned full strings"), and the reword **must preserve** those existing pinned substrings.

- [ ] **Step 1: Add the red-first assertions** to `dogfood_test.go` (after the existing description checks):

```go
	// machine→human ordering must be explicit (t62 Goal 5/§8). Only the `task`
	// type has the spec/plan review stages; `chore` has just tbd→implementing→
	// impl_review→approved (its human step is the PR merge, already after the
	// agent impl_review), so DO NOT loop {task, chore} here (chore lacks these
	// statuses → StatusByName returns a zero Status and the assertion would
	// falsely fail).
	if s, _ := task.StatusByName("spec_review"); !strings.Contains(s.Description, "agent") {
		t.Fatalf("task spec_review must name the agent gate: %q", s.Description)
	}
	if s, _ := task.StatusByName("spec_human_review"); !strings.Contains(s.Description, "after the agent") {
		t.Fatalf("task spec_human_review must state it is after the agent review: %q", s.Description)
	}
	if s, _ := task.StatusByName("plan_human_review"); !strings.Contains(s.Description, "after the agent") {
		t.Fatalf("task plan_human_review must state it is after the agent review: %q", s.Description)
	}
	// both types: the machine review status names the agent gate.
	for _, tc := range []mtt.Type{task, chore} {
		if s, _ := tc.StatusByName("impl_review"); !strings.Contains(s.Description, "agent") {
			t.Fatalf("%s impl_review must name the agent gate: %q", tc.Name, s.Description)
		}
	}
```

Run: `go test ./internal/adapter/yaml/ -run TestRepoDogfoodConfig -v` → FAIL (config lacks "after the agent").

- [ ] **Step 2: Reword `.mtt/config.yaml`** — the review statuses/edges in **both** the `task` and `chore` blocks, **preserving** every existing pinned substring. Example rewordings:
  - `spec_review` status: `"agent gate: run an adversarial subagent review of the spec; mtt approve (→ human sign-off) when it passes, mtt decline to send back"`
  - `spec_review → spec_human_review` (approve edge): `"agent review passed → human sign-off next"`
  - `spec_human_review` status: `"human sign-off (only after the agent spec_review passed); mtt approve or mtt decline"`
  - same shape for `plan_review`/`plan_human_review`; `impl_review` → `"agent gate: run an adversarial code review …"` (chore `impl_review` **keeps** its "it must be a" type-boundary line; `approved` keeps "gh pr create" / "ask the human to merge" — already after the agent gate).

- [ ] **Step 3: Run** — `go test ./internal/adapter/yaml/ -run TestRepoDogfoodConfig -v` → PASS; `make check` → green. Sanity: `./bin/mtt types` shows the reworded descriptions and still contains the preserved substrings.

- [ ] **Step 4: Commit**

```bash
git add .mtt/config.yaml internal/adapter/yaml/dogfood_test.go
git commit -m "t62: state machine->human review ordering explicitly in the flow descriptions (dogfood config)"
```

---

### Task 7: Author `templates/git-flow.yaml` (the flagship) — derived from the reworded config

Replace the Task-5 placeholder with the real flagship, derived from the now-reworded `.mtt/config.yaml`, so it inherits both the machine→human wording (Task 6) and the safety statements.

**Files:**
- Modify: `templates/git-flow.yaml` (replace the Task-5 placeholder with the real flagship)
- Test: `templates/templates_test.go`

**Interfaces:** none new (a data file + tests).

- [ ] **Step 1: Write the failing test** (`templates/templates_test.go`):

```go
package templates_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
)

func TestExamplesLoadAndValidate(t *testing.T) {
	// The hosted, verbatim-installed examples must be standalone-valid. NOT
	// default.yaml — it is the templated built-in (carries {{.Name}}, rendered
	// via text/template), so it is not valid standalone YAML; its validity is
	// covered by the adapter's render+Load golden tests.
	for _, name := range []string{"coding.yaml", "hierarchy.yaml", "git-flow.yaml"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if err := yaml.ValidateTemplateBytes(data); err != nil {
			t.Fatalf("%s must load+validate: %v", name, err)
		}
	}
}

func TestGitFlowStatesItsAssumptions(t *testing.T) {
	data, err := os.ReadFile("git-flow.yaml")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	// the "stated" half of no-silent-traps: descriptions must mention these.
	ghToken := regexp.MustCompile(`(?i)\bgh\b`) // token, not the "through"/"high" bigram
	for _, want := range []string{"push", "jq", "--no-run"} {
		if !strings.Contains(strings.ToLower(s), want) {
			t.Fatalf("git-flow.yaml must state %q (no silent traps)", want)
		}
	}
	if !ghToken.MatchString(s) {
		t.Fatal("git-flow.yaml must mention the `gh` tool as a token")
	}
	// machine→human ordering wording (inherited from the reworded config, Task 6).
	if !regexp.MustCompile(`(?i)after the agent`).MatchString(s) {
		t.Fatal("git-flow.yaml *_human_review must state it comes after the agent review")
	}
}
```

- [ ] **Step 2: Run** → FAIL (placeholder git-flow.yaml lacks the descriptions).

- [ ] **Step 3: Author `templates/git-flow.yaml`.** Derive it from the repo's now-reworded `.mtt/config.yaml` `task` type (do NOT invent a flow — copy the dogfood structure so it is battle-tested): (a) keep one `task` type, `prefix: t`, `default: true`; (b) set `project.name` to a literal `my-project` (NOT `{{.Name}}` — external templates are verbatim, uneval'd); (c) keep the statuses/transitions and their `commands`/`post`/`post_defaults` **and their descriptions** — which now carry both the pushable-main / gh+jq / squash-title / `--no-run` statements **and** the Task-6 machine→human ordering wording; (d) keep it self-contained (no repo-specific paths beyond the generic flow). Verify with `./bin/mtt` by installing into a temp dir: `mtt init --template ./templates/git-flow.yaml` in `/tmp/...` and `mtt types` renders.

- [ ] **Step 4: Run** — `go test ./templates/ -v` → PASS; `make check` → green.

- [ ] **Step 5: Commit**

```bash
git add templates/git-flow.yaml templates/templates_test.go
git commit -m "t62: author git-flow.yaml flagship (generalized reworded dogfood flow; states its assumptions + ordering)"
```

---

### Task 8: Docs sweep (EN+RU) + CLAUDE.md + CHANGELOG + close-out

**Files:**
- Modify: `CLI_REFERENCE.md` + `.ru.md`, `README.md` + `.ru.md`, `DESIGN.md` + `.ru.md`, `FLOW_GUIDE.md` + `.ru.md`, `demo/README.md` + `.ru.md`, `internal/cli/CLAUDE.md`, `internal/adapter/yaml/CLAUDE.md`, `templates/` (a new `CLAUDE.md` for the package), `CHANGELOG.md`

- [ ] **Step 1: CLI_REFERENCE (+ru)** — rewrite `mtt init`: `--template <name|path|url>` (three forms + the resolution rule); https-only + confirm + `--yes` + the 30s/1 MiB caps; validate-before-write; the ⚠ review notice; `--json` `source`; `--name` ignored for external; the built-in list is now just `default`.
- [ ] **Step 2: README (+ru)** — replace the two `--template coding`/`hierarchy` runnable mentions with `default` and a `--template <url>` example (fetch `git-flow.yaml`); no README runs a removed built-in.
- [ ] **Step 3: FLOW_GUIDE (+ru)** — the forward-link resolves: "install a ready flow: `mtt init --template <url>`" → `templates/git-flow.yaml`; update any `--template hierarchy/coding` mention.
- [ ] **Step 4: DESIGN (+ru)** — a `Shipped (t62)` note: the minimal-built-in + bring-your-own strategy, the top-level `templates/` dir, the `checkDecoded`/`ValidateTemplateBytes` boundary, the no-silent-traps / SEC2 posture, t24→t66 unblocking the flagship, and the explicit machine→human review-ordering wording.
- [ ] **Step 5: `demo/README.md` + `.ru.md`** (+ `demo/doc.go` if it names a removed built-in) — the `mtt init --template coding` prose → the migrated form (`--template ./coding.yaml`).
- [ ] **Step 6: CLAUDE.md** — `internal/cli` (init external-source flow + confirm/notice/`--yes`), `internal/adapter/yaml` (`ClassifyTemplate`/`Fetcher`/`checkDecoded`+`ValidateTemplateBytes`/verbatim; the `Load` refactor keeps the overlay), and a new `templates/CLAUDE.md` (embed `default` only; the rest hosted examples).
- [ ] **Step 7: CHANGELOG** — one `[Unreleased]` entry (the `--template <path|url>` sources, the de-embed of coding/hierarchy, the git-flow flagship, the review-ordering wording).
- [ ] **Step 8: Close-out** — `make check` green; run the AGENTS.md Principles self-check; `git grep -n "internal/adapter/yaml/templates"` returns no stale references; verify `./bin/mtt init` (no arg), `--template ./templates/coding.yaml`, and the non-TTY URL refuse by hand. Then `mtt submit`.
