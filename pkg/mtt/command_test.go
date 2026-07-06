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
	}
	for _, tc := range cases {
		if got := tc.cmd.Valid(); got != tc.want {
			t.Errorf("%s: Valid() = %v, want %v", tc.name, got, tc.want)
		}
	}
}
