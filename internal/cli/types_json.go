package cli

import (
	"fmt"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// typeJSON is the machine-readable view of a configured type and its full flow
// graph (statuses + transitions incl. gates/post/rollback/require/current).
type typeJSON struct {
	Name        string           `json:"name"`
	Prefix      string           `json:"prefix"`
	Parents     []string         `json:"parents"`
	Default     bool             `json:"default,omitempty"`
	Description string           `json:"description,omitempty"`
	Statuses    []statusJSON     `json:"statuses"`
	Transitions []transitionJSON `json:"transitions"`
}

type statusJSON struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Default     bool   `json:"default,omitempty"`
	Description string `json:"description,omitempty"`
}

type transitionJSON struct {
	Name        string        `json:"name,omitempty"`
	From        string        `json:"from"`
	To          string        `json:"to"`
	Description string        `json:"description,omitempty"`
	Current     string        `json:"current,omitempty"`
	Require     *requireJSON  `json:"require,omitempty"` // pointer: omitempty is ignored on a struct value
	Commands    []commandJSON `json:"commands"`
	Post        []commandJSON `json:"post"`
}

type commandJSON struct {
	Run      string        `json:"run"`
	Timeout  string        `json:"timeout,omitempty"`
	Rollback *rollbackJSON `json:"rollback,omitempty"`
}

type rollbackJSON struct {
	Run     string `json:"run"`
	Timeout string `json:"timeout,omitempty"`
}

type requireJSON struct {
	Who bool `json:"who,omitempty"`
	Why bool `json:"why,omitempty"`
}

// toTypesJSON maps the configured types (optionally filtered to one) to their
// JSON views. Mirrors formatTypes: an unknown filter is an error.
func toTypesJSON(cfg mtt.Config, prefixes map[string]string, filter string) ([]typeJSON, error) {
	out := make([]typeJSON, 0, len(cfg.Types))
	for _, t := range cfg.Types {
		if filter != "" && string(t.Name) != filter {
			continue
		}
		out = append(out, toTypeJSON(t, prefixes[string(t.Name)]))
	}
	if filter != "" && len(out) == 0 {
		return nil, fmt.Errorf("unknown type %q", filter)
	}
	return out, nil
}

// toTypeJSON maps one type (prefix comes from settings.Prefixes, not the domain type).
func toTypeJSON(t mtt.Type, prefix string) typeJSON {
	parents := make([]string, len(t.Parents))
	for i, p := range t.Parents {
		parents[i] = string(p)
	}
	statuses := make([]statusJSON, len(t.Statuses))
	for i, s := range t.Statuses {
		statuses[i] = statusJSON{Name: string(s.Name), Kind: string(s.Kind), Default: s.Default, Description: s.Description}
	}
	transitions := make([]transitionJSON, len(t.Transitions))
	for i, tr := range t.Transitions {
		transitions[i] = toTransitionJSON(tr)
	}
	return typeJSON{
		Name: string(t.Name), Prefix: prefix, Parents: parents,
		Default: t.Default, Description: t.Description,
		Statuses: statuses, Transitions: transitions,
	}
}

func toTransitionJSON(tr mtt.Transition) transitionJSON {
	commands := make([]commandJSON, len(tr.Commands))
	for i, c := range tr.Commands {
		commands[i] = toCommandJSON(c)
	}
	post := make([]commandJSON, len(tr.Post))
	for i, c := range tr.Post {
		post[i] = toCommandJSON(c)
	}
	var req *requireJSON
	if tr.Require.Who || tr.Require.Why {
		req = &requireJSON{Who: tr.Require.Who, Why: tr.Require.Why}
	}
	return transitionJSON{
		Name: tr.Name, From: string(tr.From), To: string(tr.To),
		Description: tr.Description, Current: string(tr.Current),
		Require: req, Commands: commands, Post: post,
	}
}

func toCommandJSON(c mtt.Command) commandJSON {
	cj := commandJSON{Run: c.Run}
	if c.Timeout > 0 {
		cj.Timeout = c.Timeout.String()
	}
	if c.Rollback != nil {
		rb := &rollbackJSON{Run: c.Rollback.Run}
		if c.Rollback.Timeout > 0 {
			rb.Timeout = c.Rollback.Timeout.String()
		}
		cj.Rollback = rb
	}
	return cj
}
