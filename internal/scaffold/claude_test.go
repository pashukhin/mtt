package scaffold

import (
	"encoding/json"
	"strings"
	"testing"
)

// createdBytes is the canonical Created output: alphabetical object keys,
// 2-space indent, trailing newline, literal 2>/dev/null (no HTML escaping).
const createdBytes = `{
  "hooks": {
    "PreCompact": [
      {
        "hooks": [
          {
            "command": "mtt prime 2>/dev/null || true",
            "type": "command"
          }
        ]
      }
    ],
    "SessionStart": [
      {
        "hooks": [
          {
            "command": "mtt prime 2>/dev/null || true",
            "type": "command"
          }
        ]
      }
    ]
  },
  "permissions": {
    "allow": [
      "Bash(mtt roadmap:*)",
      "Bash(mtt list:*)",
      "Bash(mtt show:*)",
      "Bash(mtt tree:*)",
      "Bash(mtt ready:*)",
      "Bash(mtt types:*)",
      "Bash(mtt prime:*)",
      "Bash(mtt check:*)",
      "Bash(mtt tags:*)",
      "Bash(mtt version:*)",
      "Bash(mtt dep list:*)",
      "Bash(mtt note show:*)",
      "Bash(mtt note list:*)",
      "Bash(mtt ref list:*)",
      "Bash(mtt note ref list:*)"
    ]
  }
}
`

// mustJSONString returns s as a JSON string literal (with quotes), for building
// table fixtures whose command contains quotes/backslashes safely.
func mustJSONString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func TestMergeCreatedFromAbsent(t *testing.T) {
	got, action, err := claudeHarness{}.Merge(nil, false)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if action != Created {
		t.Fatalf("action = %q, want created", action)
	}
	if string(got) != createdBytes {
		t.Fatalf("bytes mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, createdBytes)
	}
	if !strings.Contains(string(got), "2>/dev/null") {
		t.Fatal("literal 2>/dev/null must survive (no HTML escaping)")
	}
}

func TestMergeConvergence(t *testing.T) {
	// R1: the Created output is the serializer's fixed point.
	_, action, err := claudeHarness{}.Merge([]byte(createdBytes), true)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if action != Unchanged {
		t.Fatalf("action = %q, want unchanged", action)
	}
}

func TestMergeReorderConvergesInOneStep(t *testing.T) {
	// spec test 5: a hand-authored file carrying our hooks but with type-before-
	// command key order (and no permissions) ⇒ exactly one Merged (re-sort + add
	// the allowlist), then Unchanged.
	h := claudeHarness{}
	in := `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"mtt prime 2>/dev/null || true"}]}],` +
		`"PreCompact":[{"hooks":[{"type":"command","command":"mtt prime 2>/dev/null || true"}]}]}}`
	first, a1, err := h.Merge([]byte(in), true)
	if err != nil {
		t.Fatalf("Merge(1): %v", err)
	}
	if a1 != Merged {
		t.Fatalf("first = %q, want merged", a1)
	}
	_, a2, err := h.Merge(first, true)
	if err != nil {
		t.Fatalf("Merge(2): %v", err)
	}
	if a2 != Unchanged {
		t.Fatalf("second = %q, want unchanged", a2)
	}
}

func TestMergeEmptyFileTreatedAsAbsent(t *testing.T) {
	got, action, err := claudeHarness{}.Merge([]byte("   \n\t"), true)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if action != Created || string(got) != createdBytes {
		t.Fatalf("empty file must create; got action %q", action)
	}
}

func TestMergePreservesUnrelatedKeys(t *testing.T) {
	in := `{"env":{"FOO":"bar"}}`
	got, action, err := claudeHarness{}.Merge([]byte(in), true)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if action != Merged {
		t.Fatalf("action = %q, want merged", action)
	}
	if !strings.Contains(string(got), `"env"`) || !strings.Contains(string(got), `"FOO"`) {
		t.Fatalf("unrelated keys dropped:\n%s", got)
	}
	if !strings.Contains(string(got), "SessionStart") {
		t.Fatalf("our hook not added:\n%s", got)
	}
}

func TestMergeCustomPrimeArgsNoDuplicate(t *testing.T) {
	in := `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"mtt prime --limit 5"}]}],` +
		`"PreCompact":[{"hooks":[{"type":"command","command":"mtt prime --limit 5"}]}]}}`
	got, _, err := claudeHarness{}.Merge([]byte(in), true)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	// Count the exact custom command (NOT the bare "mtt prime" substring, which
	// also appears in the "Bash(mtt prime:*)" allowlist entry): the two existing
	// hooks stay, and our guarded command is NOT appended (they satisfy presence).
	if n := strings.Count(string(got), "mtt prime --limit 5"); n != 2 {
		t.Fatalf("expected the 2 existing custom prime hooks, no duplicate; got %d:\n%s", n, got)
	}
	if strings.Contains(string(got), "mtt prime 2>/dev/null") {
		t.Fatalf("our guarded hook should not be appended when a prime hook is already present:\n%s", got)
	}
}

func TestMergeWordBoundaryNotPresent(t *testing.T) {
	// r5/d4: a mention not-at-start, and a longer word, must NOT count as present;
	// our hook is appended AND the original command is preserved (spec 6).
	cases := []struct{ cmd, survives string }{
		{`git commit -m "wire mtt prime"`, "git commit -m"},
		{"mtt primer", "mtt primer"},
	}
	for _, c := range cases {
		in := `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":` +
			mustJSONString(c.cmd) + `}]}]}}`
		got, action, err := claudeHarness{}.Merge([]byte(in), true)
		if err != nil {
			t.Fatalf("Merge(%q): %v", c.cmd, err)
		}
		if action != Merged || strings.Count(string(got), "SessionStart") != 1 {
			t.Fatalf("cmd %q: our hook should be appended; action %q:\n%s", c.cmd, action, got)
		}
		if !strings.Contains(string(got), "mtt prime 2>/dev/null") {
			t.Fatalf("cmd %q: our prime command missing:\n%s", c.cmd, got)
		}
		if !strings.Contains(string(got), c.survives) {
			t.Fatalf("cmd %q: the original command must be preserved:\n%s", c.cmd, got)
		}
	}
}

func TestMergeMalformedJSON(t *testing.T) {
	// NB: a composite literal (claudeHarness{}) may not appear in an if-init
	// header (Go parse ambiguity) — hoist it to a local.
	h := claudeHarness{}
	if _, _, err := h.Merge([]byte("{not json"), true); err == nil {
		t.Fatal("malformed JSON must error")
	}
}

func TestMergeRefusesIncompatibleShape(t *testing.T) {
	h := claudeHarness{}
	for _, in := range []string{
		`{"hooks":"oops"}`,
		`{"hooks":{"SessionStart":"oops"}}`,
		`{"permissions":{"allow":"oops"}}`,
	} {
		if _, _, err := h.Merge([]byte(in), true); err == nil {
			t.Fatalf("incompatible shape %q must refuse", in)
		}
	}
}

func TestMergeNullContainerTreatedAsAbsent(t *testing.T) {
	h := claudeHarness{}
	for _, in := range []string{
		`{"hooks":null}`,
		`{"hooks":{"SessionStart":null}}`,
		`{"permissions":null}`,
		`{"permissions":{"allow":null}}`,
	} {
		if _, _, err := h.Merge([]byte(in), true); err != nil {
			t.Fatalf("null container %q must be treated as absent, got err %v", in, err)
		}
	}
}

func TestMergeToleratesOddShapes(t *testing.T) {
	// valid JSON, odd shapes the presence walk must survive without panic — and
	// (spec 12) our hook is still appended where safe.
	h := claudeHarness{}
	for _, in := range []string{
		`{"hooks":{"SessionStart":[{}]}}`,
		`{"hooks":{"SessionStart":[{"hooks":[{"command":42}]}]}}`,
	} {
		got, _, err := h.Merge([]byte(in), true)
		if err != nil {
			t.Fatalf("odd shape %q should not error, got %v", in, err)
		}
		if !strings.Contains(string(got), "mtt prime 2>/dev/null") {
			t.Fatalf("odd shape %q: our hook should be appended:\n%s", in, got)
		}
	}
}

func TestMergePermissionsUnionDedup(t *testing.T) {
	in := `{"permissions":{"allow":["Bash(mtt roadmap:*)","Custom(x)"]}}`
	got, _, err := claudeHarness{}.Merge([]byte(in), true)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if strings.Count(string(got), "Bash(mtt roadmap:*)") != 1 {
		t.Fatalf("roadmap entry should be deduped:\n%s", got)
	}
	if !strings.Contains(string(got), "Custom(x)") {
		t.Fatalf("existing custom entry dropped:\n%s", got)
	}
	// existing entries come first.
	if strings.Index(string(got), "Custom(x)") > strings.Index(string(got), "Bash(mtt list:*)") {
		t.Fatalf("existing entry should precede our new ones:\n%s", got)
	}
}

func TestMergeOneEventPresent(t *testing.T) {
	in := `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"mtt prime"}]}]}}`
	got, action, err := claudeHarness{}.Merge([]byte(in), true)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if action != Merged {
		t.Fatalf("action = %q, want merged", action)
	}
	if strings.Count(string(got), "PreCompact") != 1 {
		t.Fatalf("PreCompact should be added exactly once:\n%s", got)
	}
}
