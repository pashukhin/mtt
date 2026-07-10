// Package yaml is the default driven adapter: it stores mtt config (and later
// tasks) as YAML files under .mtt/, mints IDs, and maps its own DTOs to and from
// the pure pkg/mtt domain. It carries no business rules beyond provider-specific
// checks (prefixes, exactly-one-default).
package yaml

import (
	"errors"
	"fmt"
	"time"

	goyaml "gopkg.in/yaml.v3"

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
	Default     bool   `yaml:"default,omitempty"`
}

type ymlTransition struct {
	From        string       `yaml:"from"`
	To          string       `yaml:"to"`
	Description string       `yaml:"description"`
	Commands    []ymlCommand `yaml:"commands"`
	Current     string       `yaml:"current,omitempty"`
}

// ymlCommand is one gate command on disk. It accepts either a bare scalar (a
// command string, back-compat) or a mapping {run, timeout, rollback}; both
// collapse to a single mtt.Command. The duration is parsed here so toDomain
// stays error-free. The optional rollback is itself a ymlCommand (scalar or map).
type ymlCommand struct {
	Run      string
	Timeout  time.Duration
	Rollback *ymlCommand // nil = none
}

// UnmarshalYAML decodes a scalar command string or a {run, timeout, rollback}
// mapping. The map branch decodes into a LOCAL string-Timeout alias (never back
// into ymlCommand — that would recurse; and yaml.v3 cannot decode "30s" into a
// time.Duration) then parses the duration; the rollback field is a *ymlCommand,
// so yaml.v3 recurses into this same UnmarshalYAML for it (scalar or map).
func (c *ymlCommand) UnmarshalYAML(value *goyaml.Node) error {
	if value.Kind == goyaml.ScalarNode {
		c.Run = value.Value
		return nil
	}
	var raw struct {
		Run      string      `yaml:"run"`
		Timeout  string      `yaml:"timeout"`
		Rollback *ymlCommand `yaml:"rollback"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	c.Run = raw.Run
	if raw.Timeout != "" {
		d, err := time.ParseDuration(raw.Timeout)
		if err != nil {
			return fmt.Errorf("command %q: timeout %q: %w", raw.Run, raw.Timeout, err)
		}
		c.Timeout = d
	}
	c.Rollback = raw.Rollback
	return nil
}

// toDomain maps the command DTO to the pure domain Command, recursively copying
// the optional rollback compensator (a deep copy — a fresh *Command, not the
// DTO's pointer).
func (c ymlCommand) toDomain() mtt.Command {
	m := mtt.Command{Run: c.Run, Timeout: c.Timeout}
	if c.Rollback != nil {
		rb := c.Rollback.toDomain()
		m.Rollback = &rb
	}
	return m
}

// toDomain maps the DTO to the pure domain Config and the adapter-owned
// type-name -> prefix map.
func (yc ymlConfig) toDomain() (mtt.Config, map[string]string) {
	cfg := mtt.Config{Version: yc.Version, Project: mtt.Project{Name: yc.Project.Name}}
	prefixes := make(map[string]string, len(yc.Types))
	for _, yt := range yc.Types {
		t := mtt.Type{Name: mtt.TypeName(yt.Name), Description: yt.Description, Parents: toTypeNames(yt.Parents), Default: yt.Default}
		for _, ys := range yt.Statuses {
			t.Statuses = append(t.Statuses, mtt.Status{Name: mtt.StatusName(ys.Name), Kind: mtt.StatusKind(ys.Kind), Description: ys.Description, Default: ys.Default})
		}
		for _, yr := range yt.Transitions {
			cmds := make([]mtt.Command, 0, len(yr.Commands))
			for _, c := range yr.Commands {
				cmds = append(cmds, c.toDomain())
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
