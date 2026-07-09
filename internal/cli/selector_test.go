package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// selCmd builds a bare command carrying the selector filter flags + a stdin reader.
func selCmd(stdin string) *cobra.Command {
	c := &cobra.Command{Use: "x"}
	addSelectorFilterFlags(c)
	c.SetIn(strings.NewReader(stdin))
	return c
}

func idsEqual(t *testing.T, got []mtt.TaskID, want ...mtt.TaskID) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("ids = %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ids = %v; want %v", got, want)
		}
	}
}

func TestStripAndHasDash(t *testing.T) {
	if !hasDash([]string{"a", "-", "b"}) || hasDash([]string{"a", "b"}) {
		t.Fatal("hasDash")
	}
	idsEqual(t, toTaskIDs(stripDash([]string{"a", "-", "b"})), "a", "b")
}

func TestSelectExplicitIDs(t *testing.T) {
	c := selCmd("")
	got, err := selectTaskIDs(c, []string{"t1", "t2"}, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	idsEqual(t, got, "t1", "t2")
}

func TestSelectStdinTrimSkipDedup(t *testing.T) {
	c := selCmd("t1\n\n  t2  \nt1\n")
	got, err := selectTaskIDs(c, []string{"-"}, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	idsEqual(t, got, "t1", "t2") // trimmed, empty skipped, deduped
}

func TestSelectConflictExplicitStdin(t *testing.T) {
	c := selCmd("")
	if _, err := selectTaskIDs(c, []string{"t1", "-"}, true); err == nil ||
		!strings.Contains(err.Error(), "one source") {
		t.Fatalf("want one-source conflict, got %v", err)
	}
}

func TestSelectConflictExplicitFilter(t *testing.T) {
	c := selCmd("")
	_ = c.Flags().Set("status", "tbd")
	if _, err := selectTaskIDs(c, []string{"t1"}, true); err == nil ||
		!strings.Contains(err.Error(), "one source") {
		t.Fatalf("want one-source conflict, got %v", err)
	}
}

func TestSelectNoSource(t *testing.T) {
	c := selCmd("")
	if _, err := selectTaskIDs(c, nil, true); err == nil ||
		!strings.Contains(err.Error(), "no tasks selected") {
		t.Fatalf("want no-source error, got %v", err)
	}
}

func TestSelectTagBulkPositionalsNotASource(t *testing.T) {
	// allowExplicitIDs=false: non-dash positionals are tags, not a source, so with no
	// marker there is no source (the tag command only calls the selector in bulk-mode).
	c := selCmd("")
	if _, err := selectTaskIDs(c, []string{"backend"}, false); err == nil ||
		!strings.Contains(err.Error(), "no tasks selected") {
		t.Fatalf("want no-source, got %v", err)
	}
	// but "backend" + "-" is a single (stdin) source, no conflict:
	c2 := selCmd("t9\n")
	got, err := selectTaskIDs(c2, []string{"backend", "-"}, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	idsEqual(t, got, "t9")
}

func TestFilterActive(t *testing.T) {
	c := selCmd("")
	if filterActive(c) {
		t.Fatal("no flag changed -> inactive")
	}
	_ = c.Flags().Set("tag", "x")
	if !filterActive(c) {
		t.Fatal("--tag set -> active")
	}
}

func TestWriteIDs(t *testing.T) {
	var b strings.Builder
	if err := writeIDs(&b, []mtt.TaskID{"e1", "t2"}); err != nil {
		t.Fatal(err)
	}
	if b.String() != "e1\nt2\n" {
		t.Fatalf("writeIDs = %q", b.String())
	}
	var e strings.Builder // zero ids -> empty output (pipeline terminus)
	_ = writeIDs(&e, nil)
	if e.String() != "" {
		t.Fatalf("empty writeIDs = %q", e.String())
	}
}
