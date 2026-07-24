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

// validNamedType is a valid flow that also carries named edges (decline/approve
// out of review). Used by the named-transition invariant tests (s008.98).
func validNamedType() Type {
	return Type{
		Name: "task", Default: true,
		Flow: Flow{
			Statuses: []Status{
				{Name: "tbd", Kind: KindInitial},
				{Name: "review", Kind: KindActive},
				{Name: "fix", Kind: KindActive},
				{Name: "done", Kind: KindTerminal},
			},
			Transitions: []Transition{
				{From: "tbd", To: "review"},
				{From: "review", To: "fix", Name: "decline"},
				{From: "review", To: "done", Name: "approve"},
				{From: "fix", To: "review"},
			},
		},
	}
}

func TestValidateNamedFlowOK(t *testing.T) {
	if err := (Config{Types: []Type{validNamedType()}}).Validate(); err != nil {
		t.Fatalf("valid named flow rejected: %v", err)
	}
}

func TestValidateRejectsDuplicateEdgeNamePerSource(t *testing.T) {
	typ := validNamedType()
	// A second edge out of review also named "decline". To a self-loop so the ONLY
	// new violation is the duplicate name (a distinct (from,to), review already
	// has in/out edges so topology stays valid).
	typ.Transitions = append(typ.Transitions, Transition{From: "review", To: "review", Name: "decline"})
	err := (Config{Types: []Type{typ}}).Validate()
	if err == nil || !strings.Contains(err.Error(), "duplicate transition name") {
		t.Fatalf("want duplicate-name error, got: %v", err)
	}
}

func TestValidateRejectsEdgeNameEqualToStatusName(t *testing.T) {
	typ := validNamedType()
	typ.Transitions[0].Name = "fix" // tbd->review named "fix" collides with the status "fix"
	err := (Config{Types: []Type{typ}}).Validate()
	if err == nil || !strings.Contains(err.Error(), "collides with a status name") {
		t.Fatalf("want name/status collision error, got: %v", err)
	}
}

func TestValidateRejectsDuplicateFromTo(t *testing.T) {
	typ := validNamedType()
	typ.Transitions = append(typ.Transitions, Transition{From: "review", To: "fix", Name: "reject"})
	err := (Config{Types: []Type{typ}}).Validate()
	if err == nil || !strings.Contains(err.Error(), `duplicate transition "review"->"fix"`) {
		t.Fatalf("want duplicate-(from,to) error, got: %v", err)
	}
}

func TestValidatePostSurfacesRejectRollback(t *testing.T) {
	rb := &Command{Run: "undo"}
	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"edge post rollback rejected", func(c *Config) {
			c.Types[0].Transitions[0].Post = []Command{{Run: "x", Rollback: rb}}
		}, "post command must not carry a rollback"},
		{"post_defaults rollback rejected", func(c *Config) {
			c.Types[0].PostDefaults = []Command{{Run: "x", Rollback: rb}}
		}, "post command must not carry a rollback"},
		{"post_defaults invalid command rejected", func(c *Config) {
			c.Types[0].PostDefaults = []Command{{Run: ""}}
		}, "invalid post command"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			tc.mutate(&cfg)
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestValidateEventPostSurfaces(t *testing.T) {
	rb := &Command{Run: "undo"}
	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"event post rollback rejected", func(c *Config) {
			c.Events.Task.Create.Post = []Command{{Run: "x", Rollback: rb}}
		}, "post command must not carry a rollback"},
		{"event post invalid command rejected", func(c *Config) {
			c.Events.Note.Delete.Post = []Command{{Run: ""}}
		}, "invalid post command"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			tc.mutate(&cfg)
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() = %v, want substring %q", err, tc.want)
			}
		})
	}
}
