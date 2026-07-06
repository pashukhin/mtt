package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestFormatTypes(t *testing.T) {
	cfg := mtt.Config{Types: []mtt.Type{{
		Name: "task", Description: "A unit of work.", Default: true,
		Flow: mtt.Flow{
			Statuses: []mtt.Status{
				{Name: "tbd", Kind: mtt.KindInitial},
				{Name: "doing", Kind: mtt.KindActive},
				{Name: "done", Kind: mtt.KindTerminal},
			},
			Transitions: []mtt.Transition{
				{From: "tbd", To: "doing"},
				{From: "doing", To: "done", Description: "gate", Commands: []mtt.Command{{Run: "make test"}}},
			},
		},
	}}}
	out, err := formatTypes(cfg, map[string]string{"task": "t"}, "")
	if err != nil {
		t.Fatal(err)
	}
	want := "task  (prefix t · root · default)\n" +
		"  A unit of work.\n" +
		"  statuses: tbd[initial] doing[active] done[terminal]\n" +
		"  transitions:\n" +
		"    tbd -> doing\n" +
		"    doing -> done  # gate\n" +
		"        $ make test\n" +
		"\n"
	if out != want {
		t.Fatalf("formatTypes mismatch:\n got: %q\nwant: %q", out, want)
	}
}

func TestFormatTypesShowsCommandTimeout(t *testing.T) {
	cfg := mtt.Config{Types: []mtt.Type{{
		Name: "task", Default: true,
		Flow: mtt.Flow{
			Statuses: []mtt.Status{
				{Name: "tbd", Kind: mtt.KindInitial},
				{Name: "doing", Kind: mtt.KindActive},
				{Name: "done", Kind: mtt.KindTerminal},
			},
			Transitions: []mtt.Transition{
				{From: "tbd", To: "doing", Commands: []mtt.Command{{Run: "slow", Timeout: 30 * time.Second}}},
				{From: "doing", To: "done"},
			},
		},
	}}}
	out, err := formatTypes(cfg, map[string]string{"task": "t"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "$ slow  (timeout 30s)") {
		t.Fatalf("missing timeout annotation:\n%s", out)
	}
}

func TestFormatTypesShowsRollback(t *testing.T) {
	cfg := mtt.Config{Types: []mtt.Type{{
		Name: "task",
		Flow: mtt.Flow{
			Statuses: []mtt.Status{{Name: "tbd", Kind: mtt.KindInitial}, {Name: "doing", Kind: mtt.KindActive}},
			Transitions: []mtt.Transition{{From: "tbd", To: "doing", Commands: []mtt.Command{
				{Run: "git checkout -b x", Rollback: &mtt.Command{Run: "git branch -D x"}},
			}}},
		},
	}}}
	out, err := formatTypes(cfg, map[string]string{"task": "t"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "↩ git branch -D x") {
		t.Fatalf("types output missing the rollback annotation:\n%s", out)
	}
}

func TestFormatTypesFilter(t *testing.T) {
	cfg := mtt.Config{Types: []mtt.Type{
		{Name: "epic", Parents: nil, Flow: mtt.Flow{Statuses: []mtt.Status{{Name: "a", Kind: mtt.KindInitial}}}},
		{Name: "task", Parents: []mtt.TypeName{"epic"}},
	}}
	out, err := formatTypes(cfg, map[string]string{"epic": "e", "task": "t"}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if want := "task  (prefix t · parents: epic)\n  transitions:\n\n"; out != want {
		t.Fatalf("filtered output = %q, want %q", out, want)
	}
	if _, err := formatTypes(cfg, nil, "ghost"); err == nil {
		t.Fatal("want error for unknown type filter")
	}
}
