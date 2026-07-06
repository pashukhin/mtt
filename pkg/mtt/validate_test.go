package mtt

import (
	"strings"
	"testing"
)

// linearFlow is a minimal valid flow: initial -> active -> terminal (+ a second terminal).
func linearFlow() Flow {
	return Flow{
		Statuses: []Status{
			{Name: "tbd", Kind: KindInitial},
			{Name: "doing", Kind: KindActive},
			{Name: "done", Kind: KindTerminal},
			{Name: "cancelled", Kind: KindTerminal},
		},
		Transitions: []Transition{
			{From: "tbd", To: "doing"},
			{From: "tbd", To: "cancelled"},
			{From: "doing", To: "done"},
			{From: "doing", To: "cancelled"},
		},
	}
}

func validConfig() Config {
	return Config{Types: []Type{{Name: "task", Default: true, Flow: linearFlow()}}}
}

func TestValidateOK(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
}

func TestValidateErrors(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Config)
		want string
	}{
		{"no types", func(c *Config) { c.Types = nil }, "at least one type"},
		{"dup type", func(c *Config) { c.Types = append(c.Types, c.Types[0]) }, "duplicate type"},
		{"two defaults", func(c *Config) {
			t2 := c.Types[0]
			t2.Name = "bug"
			c.Types = append(c.Types, t2)
		}, "at most one"},
		{"unknown parent", func(c *Config) { c.Types[0].Parents = []TypeName{"ghost"} }, "unknown parent"},
		{"self parent", func(c *Config) { c.Types[0].Parents = []TypeName{"task"} }, "its own parent"},
		{"dup status", func(c *Config) {
			c.Types[0].Statuses = append(c.Types[0].Statuses, Status{Name: "tbd", Kind: KindInitial})
		}, "duplicate status"},
		{"bad kind", func(c *Config) { c.Types[0].Statuses[0].Kind = "weird" }, "invalid kind"},
		{"transition to unknown", func(c *Config) {
			c.Types[0].Transitions = append(c.Types[0].Transitions, Transition{From: "tbd", To: "ghost"})
		}, "unknown status"},
		{"no active (2-status flow)", func(c *Config) {
			c.Types[0].Flow = Flow{
				Statuses:    []Status{{Name: "a", Kind: KindInitial}, {Name: "b", Kind: KindTerminal}},
				Transitions: []Transition{{From: "a", To: "b"}},
			}
		}, "no active status"},
		{"initial with incoming", func(c *Config) {
			c.Types[0].Transitions = append(c.Types[0].Transitions, Transition{From: "doing", To: "tbd"})
		}, "initial needs 0 incoming"},
		{"two default statuses", func(c *Config) {
			c.Types[0].Statuses[0].Default = true
			c.Types[0].Statuses = append(c.Types[0].Statuses, Status{Name: "tbd2", Kind: KindInitial, Default: true})
			c.Types[0].Transitions = append(c.Types[0].Transitions, Transition{From: "tbd2", To: "doing"})
		}, "default statuses"},
		{"default on non-initial", func(c *Config) { c.Types[0].Statuses[1].Default = true }, "default status must be initial"},
		{"nested rollback", func(c *Config) {
			c.Types[0].Transitions[0].Commands = []Command{
				{Run: "a", Rollback: &Command{Run: "b", Rollback: &Command{Run: "c"}}}, // second-level rollback is invalid
			}
		}, "bad rollback"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validConfig()
			tc.mut(&c)
			err := c.Validate()
			if err == nil {
				t.Fatalf("want error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.want)
			}
		})
	}
}

func TestValidateRejectsBadCurrent(t *testing.T) {
	cfg := Config{Types: []Type{{
		Name: "task", Default: true,
		Flow: Flow{
			Statuses: []Status{
				{Name: "tbd", Kind: KindInitial}, {Name: "wip", Kind: KindActive}, {Name: "done", Kind: KindTerminal},
			},
			Transitions: []Transition{
				{From: "tbd", To: "wip"}, {From: "wip", To: "done", Current: "toggle"},
			},
		},
	}}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() = nil, want an invalid-current error")
	}
}

func TestValidateRejectsInvalidCommand(t *testing.T) {
	cfg := Config{
		Version: 1,
		Types: []Type{{
			Name: "task", Default: true,
			Flow: Flow{
				Statuses: []Status{
					{Name: "tbd", Kind: KindInitial},
					{Name: "doing", Kind: KindActive},
					{Name: "done", Kind: KindTerminal},
				},
				Transitions: []Transition{
					{From: "tbd", To: "doing", Commands: []Command{{Run: ""}}}, // empty run
					{From: "doing", To: "done"},
				},
			},
		}},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("want error for a command with an empty run")
	}
}
