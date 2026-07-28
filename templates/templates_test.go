package templates_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	yamlv3 "gopkg.in/yaml.v3"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
)

func TestExamplesLoadAndValidate(t *testing.T) {
	// The hosted, verbatim-installed examples must be standalone-valid. NOT
	// default.yaml — it is the templated built-in (carries {{.Name}}, rendered
	// via text/template), so it is not valid standalone YAML; its validity is
	// covered by the adapter's render+Load golden tests.
	for _, name := range []string{"coding.yaml", "docs.yaml", "hierarchy.yaml", "git-flow.yaml"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if err := yaml.ValidateTemplateBytes(data); err != nil {
			t.Fatalf("%s must load+validate: %v", name, err)
		}
	}
}

func TestDocsIsNoCode(t *testing.T) {
	// The docs/no-code starter must EXECUTE NOTHING — no gate, no post, no
	// lifecycle-event pipeline. A structural check (not a token grep) proves it,
	// catching any side-effecting step, not just git/gh. `events` is captured as
	// a whole so a command hidden in any store×kind slot cannot slip past; a
	// `rollback:` only nests under a `command`, so empty `commands` ⇒ no rollback.
	data, err := os.ReadFile("docs.yaml")
	if err != nil {
		t.Fatalf("read docs.yaml: %v", err)
	}
	var doc struct {
		Types []struct {
			PostDefaults []any `yaml:"post_defaults"`
			Transitions  []struct {
				From     string `yaml:"from"`
				To       string `yaml:"to"`
				Commands []any  `yaml:"commands"`
				Post     []any  `yaml:"post"`
			} `yaml:"transitions"`
		} `yaml:"types"`
		Events map[string]any `yaml:"events"`
	}
	if err := yamlv3.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal docs.yaml: %v", err)
	}
	if len(doc.Events) != 0 {
		t.Errorf("docs.yaml must define no events, got: %v", doc.Events)
	}
	for _, ty := range doc.Types {
		if len(ty.PostDefaults) != 0 {
			t.Errorf("docs.yaml must carry no post_defaults")
		}
		for _, tr := range ty.Transitions {
			if len(tr.Commands) != 0 || len(tr.Post) != 0 {
				t.Errorf("docs.yaml %s→%s must have no commands/post (no-code)", tr.From, tr.To)
			}
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
