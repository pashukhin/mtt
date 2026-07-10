package yaml

import (
	"strings"
	"testing"
	"time"

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

func TestToDomainMapsStatusDefault(t *testing.T) {
	// Two initials; the SECOND is marked default. Without the DTO mapping the
	// fallback (first initial in config order) wins — the bug A2 fixes.
	src := `
version: 1
project: {name: demo}
types:
  - name: task
    prefix: t
    parents: []
    default: true
    statuses:
      - {name: triage,  kind: initial}
      - {name: tbd,     kind: initial, default: true}
      - {name: doing,   kind: active}
      - {name: done,    kind: terminal}
    transitions:
      - {from: triage, to: doing}
      - {from: tbd,    to: doing}
      - {from: doing,  to: done}
`
	var yc ymlConfig
	if err := goyaml.Unmarshal([]byte(src), &yc); err != nil {
		t.Fatal(err)
	}
	cfg, _ := yc.toDomain()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("two-initial config invalid: %v", err)
	}
	init, ok := cfg.Types[0].InitialStatus()
	if !ok {
		t.Fatal("no initial status resolved")
	}
	if init.Name != "tbd" {
		t.Fatalf("InitialStatus = %q, want the default-marked %q (Status.Default was dropped)", init.Name, "tbd")
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

func TestToDomainTransitionCurrent(t *testing.T) {
	yc := ymlConfig{Types: []ymlType{{
		Name: "task", Prefix: "t", Default: true,
		Statuses: []ymlStatus{
			{Name: "tbd", Kind: "initial"}, {Name: "wip", Kind: "active"}, {Name: "done", Kind: "terminal"},
		},
		Transitions: []ymlTransition{
			{From: "tbd", To: "wip", Current: "set"},
			{From: "wip", To: "done", Current: "clear"},
		},
	}}}
	cfg, _ := yc.toDomain()
	trs := cfg.Types[0].Transitions
	if trs[0].Current != mtt.CurrentSet {
		t.Errorf("edge tbd->wip Current = %q, want set", trs[0].Current)
	}
	if trs[1].Current != mtt.CurrentClear {
		t.Errorf("edge wip->done Current = %q, want clear", trs[1].Current)
	}
}

func TestYmlCommandUnmarshalScalar(t *testing.T) {
	var c ymlCommand
	if err := goyaml.Unmarshal([]byte(`"make test"`), &c); err != nil {
		t.Fatal(err)
	}
	if c.Run != "make test" || c.Timeout != 0 {
		t.Fatalf("got %+v, want {Run: make test, Timeout: 0}", c)
	}
}

func TestYmlCommandUnmarshalMap(t *testing.T) {
	var c ymlCommand
	if err := goyaml.Unmarshal([]byte("{run: make test, timeout: 30s}"), &c); err != nil {
		t.Fatal(err)
	}
	if c.Run != "make test" || c.Timeout != 30*time.Second {
		t.Fatalf("got %+v, want {Run: make test, Timeout: 30s}", c)
	}
}

func TestYmlCommandUnmarshalBadDuration(t *testing.T) {
	var c ymlCommand
	if err := goyaml.Unmarshal([]byte("{run: x, timeout: banana}"), &c); err == nil {
		t.Fatal("want error for a bad duration")
	}
}

func TestToDomainCommandsMixed(t *testing.T) {
	src := `
version: 1
project: {name: p}
types:
  - name: task
    prefix: t
    default: true
    statuses:
      - {name: tbd, kind: initial}
      - {name: doing, kind: active}
      - {name: done, kind: terminal}
    transitions:
      - {from: tbd, to: doing, commands: ["make lint", {run: "make test", timeout: 30s}]}
      - {from: doing, to: done}
`
	var yc ymlConfig
	if err := goyaml.Unmarshal([]byte(src), &yc); err != nil {
		t.Fatal(err)
	}
	cfg, _ := yc.toDomain()
	cmds := cfg.Types[0].Transitions[0].Commands
	if len(cmds) != 2 {
		t.Fatalf("cmds = %+v, want 2", cmds)
	}
	if cmds[0] != (mtt.Command{Run: "make lint"}) {
		t.Fatalf("cmd0 = %+v", cmds[0])
	}
	if cmds[1] != (mtt.Command{Run: "make test", Timeout: 30 * time.Second}) {
		t.Fatalf("cmd1 = %+v", cmds[1])
	}
}

func TestYmlCommandUnmarshalRollbackScalar(t *testing.T) {
	var c ymlCommand
	if err := goyaml.Unmarshal([]byte("{run: git checkout -b x, rollback: git branch -D x}"), &c); err != nil {
		t.Fatal(err)
	}
	if c.Rollback == nil || c.Rollback.Run != "git branch -D x" || c.Rollback.Timeout != 0 {
		t.Fatalf("rollback = %+v, want {Run: git branch -D x}", c.Rollback)
	}
}

func TestYmlCommandUnmarshalRollbackMap(t *testing.T) {
	var c ymlCommand
	if err := goyaml.Unmarshal([]byte("{run: a, rollback: {run: b, timeout: 30s}}"), &c); err != nil {
		t.Fatal(err)
	}
	if c.Rollback == nil || c.Rollback.Run != "b" || c.Rollback.Timeout != 30*time.Second {
		t.Fatalf("rollback = %+v, want {Run: b, Timeout: 30s}", c.Rollback)
	}
}

func TestToDomainRollbackDeepCopy(t *testing.T) {
	yc := ymlCommand{Run: "a", Rollback: &ymlCommand{Run: "b"}}
	m := yc.toDomain()
	if m.Rollback == nil || m.Rollback.Run != "b" {
		t.Fatalf("rollback not mapped: %+v", m.Rollback)
	}
	yc.Rollback.Run = "changed" // mutating the DTO must not affect the domain copy
	if m.Rollback.Run != "b" {
		t.Fatal("toDomain aliased the rollback pointer instead of deep-copying")
	}
}
