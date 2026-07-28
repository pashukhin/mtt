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
	Require        RequireAttribution
	// ExtractHashtags is the committed-config policy for turning #hashtags in a task's
	// title/description into tags. Default OFF (absent in config.yaml → false): explicit
	// --tag / tag add is then the only tag source. The CLI threads it into the core
	// add/edit/tag usecases.
	ExtractHashtags bool
}

// RequireAttribution is the project's required-attribution policy (who/why must
// be provided on a transition). Committed in config.yaml; config.local may only
// TIGHTEN (add requirements), never relax.
type RequireAttribution struct {
	Who bool
	Why bool
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
	committedRequire := yc.Require // capture before the local overlay (tighten-only)
	if err := decodeInto(filepath.Join(root, dirName, localConfigName), &yc, false); err != nil {
		return mtt.Config{}, Settings{}, err
	}
	cfg, prefixes, timeout, err := checkDecoded(yc)
	if err != nil {
		return mtt.Config{}, Settings{}, err
	}
	require := RequireAttribution{
		Who: committedRequire.Who || yc.Require.Who,
		Why: committedRequire.Why || yc.Require.Why,
	}
	return cfg, Settings{Prefixes: prefixes, CommandTimeout: timeout, Author: yc.Author, Require: require, ExtractHashtags: yc.ExtractHashtags}, nil
}

// checkDecoded runs the YAML provider's post-decode checks — exactly what Load
// applies after its overlay: toDomain + checkPrefixes (single default, prefix
// present/unique/letters-only — the shell-safety boundary) + parseCommandTimeout.
// It deliberately does NOT run Config.Validate (that is the caller's, per Load's
// contract); the external-template path adds it in ValidateTemplateBytes.
func checkDecoded(yc ymlConfig) (mtt.Config, map[string]string, time.Duration, error) {
	cfg, prefixes := yc.toDomain()
	if err := checkPrefixes(cfg, prefixes); err != nil {
		return mtt.Config{}, nil, 0, err
	}
	timeout, err := parseCommandTimeout(yc.CommandTimeout)
	if err != nil {
		return mtt.Config{}, nil, 0, err
	}
	return cfg, prefixes, timeout, nil
}

// ValidateTemplateBytes fail-closed-validates an external (path/url) template:
// decode a single doc → the provider checks (checkDecoded) → Config.Validate.
// Init legitimately IS the caller that owns the domain check for an untrusted
// template, so this is the one place Config.Validate rides the load path.
func ValidateTemplateBytes(data []byte) error {
	var yc ymlConfig
	if err := goyaml.Unmarshal(data, &yc); err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	cfg, _, _, err := checkDecoded(yc)
	if err != nil {
		return err
	}
	return cfg.Validate()
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
