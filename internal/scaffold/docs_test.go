package scaffold

import (
	"strings"
	"testing"
)

func TestBlockMergeCreatedFromAbsent(t *testing.T) {
	got, action, err := blockMerge(nil, false, "hello")
	if err != nil {
		t.Fatalf("blockMerge: %v", err)
	}
	if action != Created {
		t.Fatalf("action = %q, want created", action)
	}
	want := "<!-- mtt:begin -->\nhello\n<!-- mtt:end -->\n"
	if string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBlockMergeEmptyTreatedAsAbsent(t *testing.T) {
	got, action, err := blockMerge([]byte("  \n\t"), true, "hi")
	if err != nil {
		t.Fatalf("blockMerge: %v", err)
	}
	if action != Created || !strings.Contains(string(got), "<!-- mtt:begin -->") {
		t.Fatalf("empty should create; action %q:\n%s", action, got)
	}
}

func TestBlockMergeAppendsPreservingContent(t *testing.T) {
	existing := []byte("# My Project\n\nSome notes.\n")
	got, action, err := blockMerge(existing, true, "runbook")
	if err != nil {
		t.Fatalf("blockMerge: %v", err)
	}
	if action != Merged {
		t.Fatalf("action = %q, want merged", action)
	}
	if !strings.HasPrefix(string(got), "# My Project\n\nSome notes.\n") {
		t.Fatalf("original content not preserved:\n%s", got)
	}
	if !strings.Contains(string(got), "<!-- mtt:begin -->\nrunbook\n<!-- mtt:end -->\n") {
		t.Fatalf("block not appended:\n%s", got)
	}
}

func TestBlockMergeConverges(t *testing.T) {
	// append then re-merge => Unchanged (one-step convergence).
	existing := []byte("# P\n")
	first, _, err := blockMerge(existing, true, "runbook")
	if err != nil {
		t.Fatalf("blockMerge(1): %v", err)
	}
	_, action, err := blockMerge(first, true, "runbook")
	if err != nil {
		t.Fatalf("blockMerge(2): %v", err)
	}
	if action != Unchanged {
		t.Fatalf("second = %q, want unchanged", action)
	}
}

func TestBlockMergeRegeneratesStale(t *testing.T) {
	existing := []byte("intro\n\n<!-- mtt:begin -->\nOLD\n<!-- mtt:end -->\ntail\n")
	got, action, err := blockMerge(existing, true, "NEW")
	if err != nil {
		t.Fatalf("blockMerge: %v", err)
	}
	if action != Merged {
		t.Fatalf("action = %q, want merged", action)
	}
	want := "intro\n\n<!-- mtt:begin -->\nNEW\n<!-- mtt:end -->\ntail\n"
	if string(got) != want {
		t.Fatalf("regenerate mismatch:\ngot  %q\nwant %q", got, want)
	}
}

func TestBlockMergeRefusesMalformed(t *testing.T) {
	for _, in := range []string{
		"x\n<!-- mtt:begin -->\nno end\n",
		"x\n<!-- mtt:end -->\nno begin\n",
		"<!-- mtt:end -->\n<!-- mtt:begin -->\nreversed\n",
		"<!-- mtt:begin -->\na\n<!-- mtt:end -->\n<!-- mtt:begin -->\nb\n<!-- mtt:end -->\n",
	} {
		if _, _, err := blockMerge([]byte(in), true, "x"); err == nil {
			t.Fatalf("malformed markers %q must refuse", in)
		}
	}
}

func TestDocTargets(t *testing.T) {
	targets := DocTargets()
	wantPaths := map[string]string{"agents": "AGENTS.md", "claude": "CLAUDE.md", "gemini": "GEMINI.md"}
	if len(targets) != 3 {
		t.Fatalf("want 3 doc targets, got %d", len(targets))
	}
	for _, tg := range targets {
		if wantPaths[tg.Name()] != tg.RelPath() {
			t.Fatalf("target %q RelPath = %q, want %q", tg.Name(), tg.RelPath(), wantPaths[tg.Name()])
		}
		got, action, err := tg.Merge(nil, false)
		if err != nil || action != Created {
			t.Fatalf("target %q create: action %q err %v", tg.Name(), action, err)
		}
		if !strings.Contains(string(got), "Working under mtt") {
			t.Fatalf("target %q content missing the runbook heading:\n%s", tg.Name(), got)
		}
	}
}
