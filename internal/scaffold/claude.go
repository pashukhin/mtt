package scaffold

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// primeCommand is the guarded session-start command (a missing mtt binary never
// nags a contributor).
const primeCommand = "mtt prime 2>/dev/null || true"

// hookEvents are the Claude Code events we hang the prime hook on: session start
// and around compaction (SessionStart also re-fires with source=compact).
var hookEvents = []string{"SessionStart", "PreCompact"}

// readOnlyAllow is the read-only mtt permission allowlist (granular per
// subcommand; every mutating verb is deliberately excluded).
var readOnlyAllow = []string{
	"Bash(mtt roadmap:*)", "Bash(mtt list:*)", "Bash(mtt show:*)", "Bash(mtt tree:*)",
	"Bash(mtt ready:*)", "Bash(mtt types:*)", "Bash(mtt prime:*)", "Bash(mtt check:*)",
	"Bash(mtt tags:*)", "Bash(mtt version:*)", "Bash(mtt dep list:*)",
	"Bash(mtt note show:*)", "Bash(mtt note list:*)", "Bash(mtt ref list:*)", "Bash(mtt note ref list:*)",
}

// claudeHarness scaffolds Claude Code's .claude/settings.json.
type claudeHarness struct{}

func (claudeHarness) Name() string    { return "claude" }
func (claudeHarness) RelPath() string { return ".claude/settings.json" }

// Merge computes the desired settings bytes from the current ones. Pure: no IO.
// Create-if-absent (or empty), additive union if present, Unchanged if already
// there, loud refusal if malformed or type-incompatible — never a clobber.
func (claudeHarness) Merge(existing []byte, exists bool) ([]byte, Action, error) {
	created := !exists || len(bytes.TrimSpace(existing)) == 0
	m := map[string]any{}
	if !created {
		if err := json.Unmarshal(existing, &m); err != nil {
			return nil, "", fmt.Errorf("parse existing settings: %w", err)
		}
	}
	if err := ensureHooks(m); err != nil {
		return nil, "", err
	}
	if err := ensurePermissions(m); err != nil {
		return nil, "", err
	}
	out, err := encode(m)
	if err != nil {
		return nil, "", err
	}
	switch {
	case created:
		return out, Created, nil
	case bytes.Equal(out, existing):
		return existing, Unchanged, nil
	default:
		return out, Merged, nil
	}
}

// encode is THE serializer for both create and merge (R1): SetEscapeHTML(false)
// keeps 2>/dev/null literal, SetIndent gives 2-space indent, Encode appends \n.
// Object keys sort alphabetically (encoding/json) — the fixed point that makes
// the Unchanged byte-equality converge.
func encode(m map[string]any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return nil, fmt.Errorf("encode settings: %w", err)
	}
	return buf.Bytes(), nil
}

// ensureHooks adds our prime hook to each event, preserving existing entries.
// A null hooks value is treated as absent; a non-null non-object refuses.
func ensureHooks(m map[string]any) error {
	hooks, err := childObject(m, "hooks")
	if err != nil {
		return err
	}
	m["hooks"] = hooks
	for _, event := range hookEvents {
		if err := ensureEventHook(hooks, event); err != nil {
			return err
		}
	}
	return nil
}

// ensureEventHook appends our matcher-group to an event array unless a prime
// hook is already present. A null event value is treated as absent; a non-null
// non-array refuses.
func ensureEventHook(hooks map[string]any, event string) error {
	arr, err := childArray(hooks, event)
	if err != nil {
		return err
	}
	if eventHasPrime(arr) {
		hooks[event] = arr
		return nil
	}
	hooks[event] = append(arr, map[string]any{
		"hooks": []any{
			map[string]any{"type": "command", "command": primeCommand},
		},
	})
	return nil
}

// ensurePermissions unions our read-only allowlist into permissions.allow,
// deduping by exact string, existing entries first.
func ensurePermissions(m map[string]any) error {
	perms, err := childObject(m, "permissions")
	if err != nil {
		return err
	}
	m["permissions"] = perms
	allow, err := childArray(perms, "allow")
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, e := range allow {
		if s, ok := e.(string); ok {
			seen[s] = true
		}
	}
	for _, entry := range readOnlyAllow {
		if !seen[entry] {
			allow = append(allow, entry)
			seen[entry] = true
		}
	}
	perms["allow"] = allow
	return nil
}

// childObject returns m[key] as an object, creating it when absent or null, and
// refusing a non-null non-object (a shape we cannot safely extend).
func childObject(m map[string]any, key string) (map[string]any, error) {
	raw, ok := m[key]
	if !ok || raw == nil {
		return map[string]any{}, nil
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("cannot merge: %q is not a JSON object", key)
	}
	return obj, nil
}

// childArray returns m[key] as an array, creating it when absent or null, and
// refusing a non-null non-array.
func childArray(m map[string]any, key string) ([]any, error) {
	raw, ok := m[key]
	if !ok || raw == nil {
		return []any{}, nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("cannot merge: %q is not a JSON array", key)
	}
	return arr, nil
}

// eventHasPrime reports whether any command under the event is our prime hook
// (word-boundary prefix, r5). Tolerates arbitrary shapes (never panics).
func eventHasPrime(arr []any) bool {
	for _, entry := range arr {
		group, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		inner, ok := group["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range inner {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			cmd, ok := hm["command"].(string)
			if ok && isPrimeCommand(cmd) {
				return true
			}
		}
	}
	return false
}

// isPrimeCommand is a word-boundary prefix match (r5/d4): the trimmed command
// equals "mtt prime" or begins with "mtt prime " (a trailing space), so
// "mtt prime --limit 5" matches but "mtt primer" and an incidental mention do not.
func isPrimeCommand(cmd string) bool {
	c := strings.TrimLeft(cmd, " \t")
	return c == "mtt prime" || strings.HasPrefix(c, "mtt prime ")
}
