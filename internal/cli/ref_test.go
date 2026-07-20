package cli

import (
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestParseRefArg(t *testing.T) {
	ok := []struct {
		in   string
		kind mtt.RefKind
		id   string
	}{
		{"task:t2", mtt.RefTask, "t2"},
		{"note:auth-design", mtt.RefNote, "auth-design"},
		{"url:https://a/b:c?x=1", mtt.RefURL, "https://a/b:c?x=1"}, // split on the FIRST colon
	}
	for _, c := range ok {
		got, err := parseRefArg(c.in)
		if err != nil || got.Kind != c.kind || got.ID != c.id {
			t.Fatalf("%q: got %+v err=%v", c.in, got, err)
		}
	}
	bad := []string{"task", "comment:t2#1", "bogus:x", "url:example.com", "task:", "note:Bad_Slug"}
	for _, in := range bad {
		if _, err := parseRefArg(in); err == nil {
			t.Fatalf("%q must error", in)
		}
	}
}
