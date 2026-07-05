package yaml

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	goyaml "gopkg.in/yaml.v3"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// defaultCommandTimeout is the per-command gate timeout when config omits
// command_timeout.
const defaultCommandTimeout = 5 * time.Minute

// Settings are the YAML adapter's non-domain, execution-level settings, returned
// alongside the pure domain Config: the type→prefix map (ID encoding), the
// per-command gate timeout, and the acting subject (Author, typically from the
// gitignored config.local overlay — the durable `by` default). Kept out of
// pkg/mtt (an external tracker adapter runs no local commands).
type Settings struct {
	Prefixes       map[string]string
	CommandTimeout time.Duration
	Author         string
}

// Load reads .mtt/config.yaml under root, merges the optional gitignored
// .mtt/config.local.yaml overlay (later layer wins at top-level-field
// granularity: a scalar like project.name overrides, but a list such as types
// replaces wholesale — yaml.v3 does not element-merge sequences), maps to the
// domain Config, and returns the adapter Settings (prefixes + command timeout)
// after the YAML provider's checks (exactly one default; prefixes present+unique).
// Domain invariants (Config.Validate) are the caller's.
func Load(root string) (mtt.Config, Settings, error) {
	var yc ymlConfig
	if err := decodeInto(filepath.Join(root, dirName, configName), &yc, true); err != nil {
		return mtt.Config{}, Settings{}, err
	}
	if err := decodeInto(filepath.Join(root, dirName, localConfigName), &yc, false); err != nil {
		return mtt.Config{}, Settings{}, err
	}
	cfg, prefixes := yc.toDomain()
	if err := checkPrefixes(cfg, prefixes); err != nil {
		return mtt.Config{}, Settings{}, err
	}
	timeout, err := parseCommandTimeout(yc.CommandTimeout)
	if err != nil {
		return mtt.Config{}, Settings{}, err
	}
	return cfg, Settings{Prefixes: prefixes, CommandTimeout: timeout, Author: yc.Author}, nil
}

// parseCommandTimeout parses the command_timeout string; empty yields the
// built-in default.
func parseCommandTimeout(s string) (time.Duration, error) {
	if s == "" {
		return defaultCommandTimeout, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("command_timeout %q: %w", s, err)
	}
	return d, nil
}

// decodeInto decodes a YAML file onto dst, overlaying whatever dst already holds.
// A missing file is not an error when required is false.
func decodeInto(path string, dst *ymlConfig, required bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if !required && errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := goyaml.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}
