package yaml

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	goyaml "gopkg.in/yaml.v3"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// Load reads .mtt/config.yaml under root, merges the optional gitignored
// .mtt/config.local.yaml overlay (later layer wins at top-level-field
// granularity: a scalar like project.name overrides, but a list such as types
// replaces wholesale — yaml.v3 does not element-merge sequences), maps to the
// domain Config, and runs the YAML provider's checks (exactly one default;
// prefixes present+unique). Domain invariants (Config.Validate) are the caller's.
func Load(root string) (mtt.Config, map[string]string, error) {
	var yc ymlConfig
	if err := decodeInto(filepath.Join(root, dirName, configName), &yc, true); err != nil {
		return mtt.Config{}, nil, err
	}
	if err := decodeInto(filepath.Join(root, dirName, localConfigName), &yc, false); err != nil {
		return mtt.Config{}, nil, err
	}
	cfg, prefixes := yc.toDomain()
	if err := checkPrefixes(cfg, prefixes); err != nil {
		return mtt.Config{}, nil, err
	}
	return cfg, prefixes, nil
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
