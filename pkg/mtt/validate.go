package mtt

import (
	"errors"
	"fmt"
)

// Validate checks the structural, name-agnostic domain invariants and returns a
// joined error listing every violation (nil when the config is valid).
func (c Config) Validate() error {
	var errs []error
	if len(c.Types) == 0 {
		errs = append(errs, errors.New("config: at least one type is required"))
	}
	seen := make(map[TypeName]bool, len(c.Types))
	defaults := 0
	for _, t := range c.Types {
		if seen[t.Name] {
			errs = append(errs, fmt.Errorf("type %q: duplicate type name", t.Name))
		}
		seen[t.Name] = true
		if t.Default {
			defaults++
		}
		errs = append(errs, t.validateFlow()...)
	}
	if defaults > 1 {
		errs = append(errs, fmt.Errorf("config: %d types marked default, want at most one", defaults))
	}
	for _, t := range c.Types {
		for _, p := range t.Parents {
			switch {
			case p == t.Name:
				errs = append(errs, fmt.Errorf("type %q: cannot be its own parent", t.Name))
			case !seen[p]:
				errs = append(errs, fmt.Errorf("type %q: unknown parent type %q", t.Name, p))
			}
		}
	}
	return errors.Join(errs...)
}

// validatePostCommands checks a post-phase pipeline: each command well-formed
// AND rollback-free — post pipelines have no compensation phase (uniform rule
// across edge post, post_defaults, and event post; t66).
func validatePostCommands(cmds []Command, where string) []error {
	var errs []error
	for _, cmd := range cmds {
		if !cmd.Valid() {
			errs = append(errs, fmt.Errorf("%s: invalid post command (empty run or negative timeout)", where))
		}
		if cmd.Rollback != nil {
			errs = append(errs, fmt.Errorf("%s: post command must not carry a rollback (post has no compensation phase)", where))
		}
	}
	return errs
}

// validateFlow checks one type's flow: status-name uniqueness, kind validity,
// transition reference resolution, kind<->topology consistency, and >=1 of each kind.
func (t Type) validateFlow() []error {
	var errs []error
	known := make(map[StatusName]bool, len(t.Statuses))
	for _, s := range t.Statuses {
		if known[s.Name] {
			errs = append(errs, fmt.Errorf("type %q: duplicate status %q", t.Name, s.Name))
		}
		known[s.Name] = true
		if !s.Kind.Valid() {
			errs = append(errs, fmt.Errorf("type %q status %q: invalid kind %q", t.Name, s.Name, s.Kind))
		}
	}
	in := make(map[StatusName]int)
	out := make(map[StatusName]int)
	pairs := make(map[string]bool)                    // (from,to) uniqueness per type
	edgeNames := make(map[StatusName]map[string]bool) // edge-name uniqueness per source status
	for _, tr := range t.Transitions {
		if !known[tr.From] {
			errs = append(errs, fmt.Errorf("type %q: transition from unknown status %q", t.Name, tr.From))
		}
		if !known[tr.To] {
			errs = append(errs, fmt.Errorf("type %q: transition to unknown status %q", t.Name, tr.To))
		}
		if !tr.Current.Valid() {
			errs = append(errs, fmt.Errorf("type %q transition %q->%q: invalid current action %q", t.Name, tr.From, tr.To, tr.Current))
		}
		for _, cmd := range tr.Commands {
			if !cmd.Valid() {
				errs = append(errs, fmt.Errorf("type %q transition %q->%q: invalid command (empty/negative timeout or bad rollback)", t.Name, tr.From, tr.To))
			}
		}
		errs = append(errs, validatePostCommands(tr.Post, fmt.Sprintf("type %q transition %q->%q", t.Name, tr.From, tr.To))...)
		key := string(tr.From) + "->" + string(tr.To)
		if pairs[key] {
			errs = append(errs, fmt.Errorf("type %q: duplicate transition %q->%q", t.Name, tr.From, tr.To))
		}
		pairs[key] = true
		if tr.Name != "" {
			if known[StatusName(tr.Name)] {
				errs = append(errs, fmt.Errorf("type %q transition %q->%q: name %q collides with a status name", t.Name, tr.From, tr.To, tr.Name))
			}
			if edgeNames[tr.From] == nil {
				edgeNames[tr.From] = make(map[string]bool)
			}
			if edgeNames[tr.From][tr.Name] {
				errs = append(errs, fmt.Errorf("type %q status %q: duplicate transition name %q", t.Name, tr.From, tr.Name))
			}
			edgeNames[tr.From][tr.Name] = true
		}
		out[tr.From]++
		in[tr.To]++
	}
	errs = append(errs, validatePostCommands(t.PostDefaults, fmt.Sprintf("type %q post_defaults", t.Name))...)
	var haveInitial, haveActive, haveTerminal bool
	for _, s := range t.Statuses {
		switch s.Kind {
		case KindInitial:
			haveInitial = true
			if in[s.Name] != 0 || out[s.Name] < 1 {
				errs = append(errs, fmt.Errorf("type %q status %q: initial needs 0 incoming and >=1 outgoing (in=%d out=%d)", t.Name, s.Name, in[s.Name], out[s.Name]))
			}
		case KindActive:
			haveActive = true
			if in[s.Name] < 1 || out[s.Name] < 1 {
				errs = append(errs, fmt.Errorf("type %q status %q: active needs >=1 incoming and >=1 outgoing (in=%d out=%d)", t.Name, s.Name, in[s.Name], out[s.Name]))
			}
		case KindTerminal:
			haveTerminal = true
			if out[s.Name] != 0 || in[s.Name] < 1 {
				errs = append(errs, fmt.Errorf("type %q status %q: terminal needs 0 outgoing and >=1 incoming (in=%d out=%d)", t.Name, s.Name, in[s.Name], out[s.Name]))
			}
		}
	}
	if !haveInitial {
		errs = append(errs, fmt.Errorf("type %q: no initial status", t.Name))
	}
	if !haveActive {
		errs = append(errs, fmt.Errorf("type %q: no active status", t.Name))
	}
	if !haveTerminal {
		errs = append(errs, fmt.Errorf("type %q: no terminal status", t.Name))
	}
	defaults := 0
	for _, s := range t.Statuses {
		if !s.Default {
			continue
		}
		defaults++
		if s.Kind != KindInitial {
			errs = append(errs, fmt.Errorf("type %q status %q: default status must be initial", t.Name, s.Name))
		}
	}
	if defaults > 1 {
		errs = append(errs, fmt.Errorf("type %q: %d default statuses, want at most one", t.Name, defaults))
	}
	return errs
}
