// Package templates holds mtt's init config templates as plain files so they are
// URL-fetchable (raw.githubusercontent.com/…/templates/<name>.yaml). Only the
// minimal built-in `default` is embedded in the binary; the rest are hosted
// examples (coding, hierarchy, git-flow) — de-embedded per the t62 minimal-
// built-in strategy.
package templates

import _ "embed"

//go:embed default.yaml
var defaultYAML []byte

// Default returns the embedded default template bytes (the sole built-in).
func Default() []byte { return defaultYAML }

// BuiltinNames returns the compiled-in template names (only "default").
func BuiltinNames() []string { return []string{"default"} }
