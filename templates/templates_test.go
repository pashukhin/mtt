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
	for _, name := range []string{"coding.yaml", "hierarchy.yaml", "git-flow.yaml", "github-gated.yaml"} {
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

func TestGitHubGatedStatesItsAssumptions(t *testing.T) {
	data, err := os.ReadFile("github-gated.yaml")
	if err != nil {
		t.Fatal(err)
	}
	s := strings.ToLower(string(data))
	// no silent traps + enforcement honesty: the header/edges must STATE every external
	// assumption and honesty limit, so a future edit that drops one fails CI.
	// `gh` as a word (not the "caught"/"through" bigram — mirrors TestGitFlowStatesItsAssumptions).
	if !regexp.MustCompile(`\bgh\b`).MatchString(s) {
		t.Fatal("github-gated.yaml must mention the `gh` tool as a token")
	}
	for _, want := range []string{"jq", "--no-run", "drift", "author", "bypass"} {
		if !strings.Contains(s, want) {
			t.Fatalf("github-gated.yaml must state %q (no silent traps / enforcement honesty)", want)
		}
	}
}
