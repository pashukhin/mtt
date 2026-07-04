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
	seen := make(map[string]bool, len(c.Types))
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

// validateFlow checks one type's flow: status-name uniqueness, kind validity,
// transition reference resolution, kind<->topology consistency, and >=1 of each kind.
func (t Type) validateFlow() []error {
	var errs []error
	known := make(map[string]bool, len(t.Statuses))
	for _, s := range t.Statuses {
		if known[s.Name] {
			errs = append(errs, fmt.Errorf("type %q: duplicate status %q", t.Name, s.Name))
		}
		known[s.Name] = true
		if !s.Kind.Valid() {
			errs = append(errs, fmt.Errorf("type %q status %q: invalid kind %q", t.Name, s.Name, s.Kind))
		}
	}
	in := make(map[string]int)
	out := make(map[string]int)
	for _, tr := range t.Transitions {
		if !known[tr.From] {
			errs = append(errs, fmt.Errorf("type %q: transition from unknown status %q", t.Name, tr.From))
		}
		if !known[tr.To] {
			errs = append(errs, fmt.Errorf("type %q: transition to unknown status %q", t.Name, tr.To))
		}
		out[tr.From]++
		in[tr.To]++
	}
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
