package yaml

import (
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// TestRepoDogfoodConfig is the SOLE guard of this repo's committed
// .mtt/config.yaml: Config.Validate runs on add/types, never on Load, so a
// broken committed config would ship silently otherwise. It locates the repo
// root (FindRoot walks up from this package dir), Load+Validate, and asserts the
// single `task` type's full 15-status gated flow with EXACT gate command strings
// (a YAML-mangled or inverted gate must fail HERE, not at runtime).
func TestRepoDogfoodConfig(t *testing.T) {
	root, err := FindRoot(".")
	if err != nil {
		t.Fatalf("FindRoot: %v", err)
	}
	cfg, settings, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if len(cfg.Types) != 1 {
		t.Fatalf("types = %d, want 1", len(cfg.Types))
	}
	task := cfg.Types[0]
	if task.Name != "task" {
		t.Fatalf("type name = %q, want task", task.Name)
	}
	if !task.Default {
		t.Fatalf("task type must be default")
	}
	if settings.Prefixes["task"] != "t" {
		t.Fatalf("prefix = %q, want t", settings.Prefixes["task"])
	}
	if !settings.Require.Who {
		t.Fatalf("require.who must be true")
	}

	wantKind := map[mtt.StatusName]mtt.StatusKind{
		"tbd":               mtt.KindInitial,
		"speccing":          mtt.KindActive,
		"spec_review":       mtt.KindActive,
		"spec_human_review": mtt.KindActive,
		"spec_fix":          mtt.KindActive,
		"planning":          mtt.KindActive,
		"plan_review":       mtt.KindActive,
		"plan_human_review": mtt.KindActive,
		"plan_fix":          mtt.KindActive,
		"implementing":      mtt.KindActive,
		"impl_review":       mtt.KindActive,
		"impl_human_review": mtt.KindActive,
		"impl_fix":          mtt.KindActive,
		"done":              mtt.KindTerminal,
		"cancelled":         mtt.KindTerminal,
	}
	if len(task.Statuses) != len(wantKind) {
		t.Fatalf("statuses = %d, want %d", len(task.Statuses), len(wantKind))
	}
	for name, kind := range wantKind {
		s, ok := task.StatusByName(name)
		if !ok {
			t.Fatalf("status %q missing", name)
		}
		if s.Kind != kind {
			t.Fatalf("status %q kind = %q, want %q", name, s.Kind, kind)
		}
	}

	edge := func(from, to mtt.StatusName) mtt.Transition {
		tr, ok := task.FindTransition(from, to)
		if !ok {
			t.Fatalf("edge %s -> %s missing", from, to)
		}
		return tr
	}
	run1 := func(tr mtt.Transition) string {
		if len(tr.Commands) != 1 {
			t.Fatalf("edge %s->%s: %d commands, want 1", tr.From, tr.To, len(tr.Commands))
		}
		return tr.Commands[0].Run
	}

	// entry: named `start`, current:set, EXACT idempotent branch command
	start := edge("tbd", "speccing")
	if start.Name != "start" || start.Current != mtt.CurrentSet {
		t.Fatalf("entry edge = {name:%q current:%q}, want {start set}", start.Name, start.Current)
	}
	if got := run1(start); got != `git switch -c task/{{.ID}} || git switch task/{{.ID}}` {
		t.Fatalf("entry branch command = %q", got)
	}

	// spec/plan submit edges carry the EXACT proxy; named `submit`
	const proxy = `git status --porcelain | grep -qv '\.mtt/'`
	for _, e := range [][2]mtt.StatusName{
		{"speccing", "spec_review"}, {"spec_fix", "spec_review"},
		{"planning", "plan_review"}, {"plan_fix", "plan_review"},
	} {
		tr := edge(e[0], e[1])
		if tr.Name != "submit" {
			t.Fatalf("edge %s->%s name = %q, want submit", e[0], e[1], tr.Name)
		}
		if got := run1(tr); got != proxy {
			t.Fatalf("edge %s->%s gate = %q, want proxy", e[0], e[1], got)
		}
	}

	// impl-review edges gate on EXACT `make check`
	for _, e := range [][2]mtt.StatusName{
		{"implementing", "impl_review"}, {"impl_fix", "impl_review"},
	} {
		tr := edge(e[0], e[1])
		if got := run1(tr); got != "make check" {
			t.Fatalf("edge %s->%s gate = %q, want make check", e[0], e[1], got)
		}
	}

	// done edge: named `approve`, current:clear
	toDone := edge("impl_human_review", "done")
	if toDone.Name != "approve" || toDone.Current != mtt.CurrentClear {
		t.Fatalf("done edge = {name:%q current:%q}, want {approve clear}", toDone.Name, toDone.Current)
	}

	// review forks are named approve/decline (spot-check the design stage)
	if tr := edge("spec_review", "spec_human_review"); tr.Name != "approve" {
		t.Fatalf("spec_review->spec_human_review name = %q, want approve", tr.Name)
	}
	if tr := edge("spec_review", "spec_fix"); tr.Name != "decline" {
		t.Fatalf("spec_review->spec_fix name = %q, want decline", tr.Name)
	}

	// no forward-trap: cancel fires from the three _fix statuses
	for _, from := range []mtt.StatusName{"spec_fix", "plan_fix", "impl_fix"} {
		if tr := edge(from, "cancelled"); tr.Name != "cancel" {
			t.Fatalf("%s->cancelled name = %q, want cancel", from, tr.Name)
		}
	}
}
