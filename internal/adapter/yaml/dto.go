// Package yaml is the default driven adapter: it stores mtt config (and later
// tasks) as YAML files under .mtt/, mints IDs, and maps its own DTOs to and from
// the pure pkg/mtt domain. It carries no business rules beyond provider-specific
// checks (prefixes, exactly-one-default).
package yaml

import (
	"errors"
	"fmt"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// ymlConfig and friends are the on-disk DTOs: they hold the yaml tags and the
// adapter-only prefix, and are mapped to the domain by toDomain.
type ymlConfig struct {
	Version        int        `yaml:"version"`
	Project        ymlProject `yaml:"project"`
	CommandTimeout string     `yaml:"command_timeout,omitempty"`
	Author         string     `yaml:"author,omitempty"`
	Require        ymlRequire `yaml:"require,omitempty"`
	Types          []ymlType  `yaml:"types"`
}

// ymlRequire is the on-disk required-attribution policy (who/why must be given
// on a transition). Committed in config.yaml; config.local may only tighten.
type ymlRequire struct {
	Who bool `yaml:"who,omitempty"`
	Why bool `yaml:"why,omitempty"`
}

type ymlProject struct {
	Name string `yaml:"name"`
}

type ymlType struct {
	Name        string          `yaml:"name"`
	Description string          `yaml:"description"`
	Prefix      string          `yaml:"prefix"`
	Parents     []string        `yaml:"parents"`
	Default     bool            `yaml:"default"`
	Statuses    []ymlStatus     `yaml:"statuses"`
	Transitions []ymlTransition `yaml:"transitions"`
}

type ymlStatus struct {
	Name        string `yaml:"name"`
	Kind        string `yaml:"kind"`
	Description string `yaml:"description"`
}

type ymlTransition struct {
	From        string   `yaml:"from"`
	To          string   `yaml:"to"`
	Description string   `yaml:"description"`
	Commands    []string `yaml:"commands"`
	Current     string   `yaml:"current,omitempty"`
}

// toDomain maps the DTO to the pure domain Config and the adapter-owned
// type-name -> prefix map.
func (yc ymlConfig) toDomain() (mtt.Config, map[string]string) {
	cfg := mtt.Config{Version: yc.Version, Project: mtt.Project{Name: yc.Project.Name}}
	prefixes := make(map[string]string, len(yc.Types))
	for _, yt := range yc.Types {
		t := mtt.Type{Name: mtt.TypeName(yt.Name), Description: yt.Description, Parents: toTypeNames(yt.Parents), Default: yt.Default}
		for _, ys := range yt.Statuses {
			t.Statuses = append(t.Statuses, mtt.Status{Name: mtt.StatusName(ys.Name), Kind: mtt.StatusKind(ys.Kind), Description: ys.Description})
		}
		for _, yr := range yt.Transitions {
			cmds := make([]mtt.Command, 0, len(yr.Commands))
			for _, run := range yr.Commands {
				cmds = append(cmds, mtt.Command{Run: run})
			}
			t.Transitions = append(t.Transitions, mtt.Transition{From: mtt.StatusName(yr.From), To: mtt.StatusName(yr.To), Description: yr.Description, Commands: cmds, Current: mtt.CurrentAction(yr.Current)})
		}
		cfg.Types = append(cfg.Types, t)
		prefixes[yt.Name] = yt.Prefix
	}
	return cfg, prefixes
}

// toTypeNames maps on-disk parent-type strings to typed names.
func toTypeNames(names []string) []mtt.TypeName {
	if len(names) == 0 {
		return nil
	}
	out := make([]mtt.TypeName, len(names))
	for i, n := range names {
		out[i] = mtt.TypeName(n)
	}
	return out
}

// checkPrefixes enforces the YAML provider's stricter rules: exactly one default
// type, and a present + unique prefix per type.
func checkPrefixes(cfg mtt.Config, prefixes map[string]string) error {
	var errs []error
	defaults := 0
	for _, t := range cfg.Types {
		if t.Default {
			defaults++
		}
	}
	if defaults != 1 {
		errs = append(errs, fmt.Errorf("config: %d types marked default, want exactly one", defaults))
	}
	seen := make(map[string]string, len(prefixes))
	for _, t := range cfg.Types {
		p := prefixes[string(t.Name)]
		if p == "" {
			errs = append(errs, fmt.Errorf("type %q: missing prefix", t.Name))
			continue
		}
		if other, dup := seen[p]; dup {
			errs = append(errs, fmt.Errorf("type %q: prefix %q already used by type %q", t.Name, p, other))
		}
		seen[p] = string(t.Name)
	}
	return errors.Join(errs...)
}
