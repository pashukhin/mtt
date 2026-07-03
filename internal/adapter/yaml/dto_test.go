package yaml

import (
	"strings"
	"testing"

	goyaml "gopkg.in/yaml.v3"

	"github.com/pashukhin/mtt/pkg/mtt"
)

const sampleConfig = `
version: 1
project: {name: demo}
types:
  - name: task
    description: A unit of work.
    prefix: t
    parents: []
    default: true
    statuses:
      - {name: tbd, kind: initial}
      - {name: doing, kind: active}
      - {name: done, kind: terminal}
    transitions:
      - {from: tbd, to: doing}
      - {from: doing, to: done}
`

func TestToDomain(t *testing.T) {
	var yc ymlConfig
	if err := goyaml.Unmarshal([]byte(sampleConfig), &yc); err != nil {
		t.Fatal(err)
	}
	cfg, prefixes := yc.toDomain()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("mapped config invalid: %v", err)
	}
	if prefixes["task"] != "t" {
		t.Fatalf("prefix = %q, want t", prefixes["task"])
	}
	if cfg.Project.Name != "demo" {
		t.Fatalf("project name = %q, want demo", cfg.Project.Name)
	}
	if cfg.Types[0].Statuses[1].Kind != mtt.KindActive {
		t.Fatalf("status kind not mapped: %q", cfg.Types[0].Statuses[1].Kind)
	}
	if err := checkPrefixes(cfg, prefixes); err != nil {
		t.Fatalf("checkPrefixes rejected a good config: %v", err)
	}
}

func TestCheckPrefixes(t *testing.T) {
	cfg := mtt.Config{Types: []mtt.Type{{Name: "a", Default: true}, {Name: "b"}}}
	if err := checkPrefixes(cfg, map[string]string{"a": "x", "b": "x"}); err == nil || !strings.Contains(err.Error(), "already used") {
		t.Fatalf("duplicate prefix not reported: %v", err)
	}
	if err := checkPrefixes(cfg, map[string]string{"a": "x", "b": ""}); err == nil || !strings.Contains(err.Error(), "missing prefix") {
		t.Fatalf("missing prefix not reported: %v", err)
	}
	noDefault := mtt.Config{Types: []mtt.Type{{Name: "a"}, {Name: "b"}}}
	if err := checkPrefixes(noDefault, map[string]string{"a": "x", "b": "y"}); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("missing default not reported: %v", err)
	}
}
