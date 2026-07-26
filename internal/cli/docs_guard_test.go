package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// docFile reads a repo-root doc by name (repoRoot anchors via runtime.Caller,
// so cwd does not matter), failing the test if absent.
func docFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(), name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

// registryCommands lists the live top-level command names. Cobra's lazily
// injected help/completion are absent because we never call Execute — so the
// test asserts only over our explicitly registered commands.
func registryCommands() []string {
	var names []string
	for _, c := range NewRootCmd().Commands() {
		names = append(names, c.Name())
	}
	return names
}

// knownVerbs = registry commands + cobra built-ins (real but lazily injected,
// so absent from NewRootCmd().Commands()) + shipped default-template statuses
// (verb sugar) + an explicit allowlist of documented non-registry verbs
// (parked/planned/edge-sugar). MAINTENANCE POINT, not zero-cost: a new
// parked/planned/edge-sugar verb mentioned in README/CLI_REFERENCE/CHANGELOG
// must be added here (a deliberate forcing function that also blocks phantoms).
// NOTE: the scan covers released CHANGELOG sections too, so a future *rename*
// of a real command would re-trip on historical entries — resolve by adding the
// old name here (an intended, visible signal), never by rewriting history.
func knownVerbs() map[string]bool {
	k := map[string]bool{}
	for _, n := range registryCommands() {
		k[n] = true
	}
	for _, s := range []string{"help", "completion"} { // cobra built-ins: real commands, documented, lazily added
		k[s] = true
	}
	for _, s := range []string{"done", "in_progress", "cancelled"} { // default-template statuses (verb sugar)
		k[s] = true
	}
	for _, s := range []string{ // parked / planned / edge-verb sugar, documented honestly
		"advance", "start", "cancel", "caps", "comment", "search", "gantt", "decline", "reparent",
	} {
		k[s] = true
	}
	return k
}

// TestDocsCommandsDocumented: every shipped command has a CLI_REFERENCE heading
// whose backtick usage line begins with `mtt <name>` (first whitespace token).
// Group commands (dep/ref/note/agent/tag) have only subcommand headings
// (`mtt dep add …`) — their first token is still the group name. Catches a
// shipped-but-undocumented command.
func TestDocsCommandsDocumented(t *testing.T) {
	ref := docFile(t, "CLI_REFERENCE.md")
	headingRe := regexp.MustCompile("(?m)^#{2,4} `mtt ([^`]+)`")
	documented := map[string]bool{}
	for _, m := range headingRe.FindAllStringSubmatch(ref, -1) {
		if f := strings.Fields(m[1]); len(f) > 0 {
			documented[f[0]] = true // first token = (group) command name; "tag" != "tags"
		}
	}
	for _, name := range registryCommands() {
		if !documented[name] {
			t.Errorf("command %q is not documented in CLI_REFERENCE.md (no `### `mtt %s …`` heading)", name, name)
		}
	}
}

// TestDocsNoPhantomCommands: every `mtt <verb>` token in the tool-describing
// docs names a known verb. FLOW_GUIDE/.ru are EXCLUDED by design — that guide
// teaches authoring custom flows and legitimately contains arbitrary
// illustrative verbs (`mtt doing`, …) indistinguishable from a phantom by
// shape. This documented cap keeps the test honest. Catches `mtt guide`.
func TestDocsNoPhantomCommands(t *testing.T) {
	known := knownVerbs()
	// Capture the first verb-word after "`mtt " (a backtick, "mtt", a space).
	// Underscore admits in_progress; hyphen admits self-update. A leading
	// [a-z] excludes "<status>"/"--flag" placeholders.
	tokenRe := regexp.MustCompile("`mtt ([a-z][a-z0-9_-]*)")
	for _, f := range []string{"README.md", "README.ru.md", "CLI_REFERENCE.md", "CLI_REFERENCE.ru.md", "CHANGELOG.md"} {
		body := docFile(t, f)
		for _, m := range tokenRe.FindAllStringSubmatch(body, -1) {
			if !known[m[1]] {
				t.Errorf("%s: `mtt %s` is not a known command/status/allowlisted verb (phantom?)", f, m[1])
			}
		}
	}
}

// TestDocsNoHardcodedVersion: the READMEs' status blockquote carries no pinned
// x.y.z (the version is ldflags-injected, so a pinned badge is stale by
// construction). Scoped to `>`-blockquote lines (the only place a badge lives).
func TestDocsNoHardcodedVersion(t *testing.T) {
	semver := regexp.MustCompile(`\d+\.\d+\.\d+`)
	for _, f := range []string{"README.md", "README.ru.md"} {
		for _, line := range strings.Split(docFile(t, f), "\n") {
			if !strings.HasPrefix(line, ">") {
				continue
			}
			if v := semver.FindString(line); v != "" {
				t.Errorf("%s: hardcoded version %q in a status blockquote — use version-agnostic wording", f, v)
			}
		}
	}
}
