package mtt

import (
	"testing"
	"time"
)

func TestCommandValid(t *testing.T) {
	cases := []struct {
		name string
		cmd  Command
		want bool
	}{
		{"run only", Command{Run: "make test"}, true},
		{"run + timeout", Command{Run: "make test", Timeout: 30 * time.Second}, true},
		{"empty run", Command{Run: ""}, false},
		{"negative timeout", Command{Run: "make test", Timeout: -1}, false},
		{"rollback leaf", Command{Run: "git checkout -b x", Rollback: &Command{Run: "git branch -D x"}}, true},
		{"rollback with timeout", Command{Run: "a", Rollback: &Command{Run: "b", Timeout: 5 * time.Second}}, true},
		{"rollback empty run", Command{Run: "a", Rollback: &Command{Run: ""}}, false},
		{"rollback negative timeout", Command{Run: "a", Rollback: &Command{Run: "b", Timeout: -1}}, false},
		{"nested rollback rejected", Command{Run: "a", Rollback: &Command{Run: "b", Rollback: &Command{Run: "c"}}}, false},
	}
	for _, tc := range cases {
		if got := tc.cmd.Valid(); got != tc.want {
			t.Errorf("%s: Valid() = %v, want %v", tc.name, got, tc.want)
		}
	}
}
