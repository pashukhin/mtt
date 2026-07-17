package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func TestToTypeJSON(t *testing.T) {
	cfg := mtt.Type{
		Name:        "task",
		Description: "a unit of work",
		Parents:     nil,
		Default:     true,
		Flow: mtt.Flow{
			Statuses: []mtt.Status{
				{Name: "tbd", Kind: mtt.KindInitial, Default: true, Description: "queued"},
				{Name: "done", Kind: mtt.KindTerminal},
			},
			Transitions: []mtt.Transition{
				{
					Name: "start", From: "tbd", To: "done", Description: "go",
					Current: mtt.CurrentSet,
					Require: mtt.Require{Who: true},
					Commands: []mtt.Command{
						{Run: "make check", Timeout: 10 * time.Minute,
							Rollback: &mtt.Command{Run: "git reset"}},
					},
					Post: []mtt.Command{{Run: "git push"}},
				},
			},
		},
	}
	v := toTypeJSON(cfg, "t")
	if v.Name != "task" || v.Prefix != "t" || !v.Default {
		t.Fatalf("head: %+v", v)
	}
	if v.Parents == nil || v.Statuses == nil || v.Transitions == nil {
		t.Fatalf("structural arrays must be non-nil: %+v", v)
	}
	if v.Statuses[0].Default != true || v.Statuses[0].Description != "queued" {
		t.Fatalf("status default/description dropped: %+v", v.Statuses[0])
	}
	tr := v.Transitions[0]
	if tr.Name != "start" || tr.From != "tbd" || tr.To != "done" || tr.Current != "set" {
		t.Fatalf("transition head: %+v", tr)
	}
	if tr.Require == nil || tr.Require.Who != true || tr.Require.Why != false {
		t.Fatalf("require must be a non-nil pointer with who=true: %+v", tr.Require)
	}
	if tr.Commands == nil || tr.Post == nil {
		t.Fatalf("commands/post must be non-nil: %+v", tr)
	}
	c := tr.Commands[0]
	if c.Run != "make check" || c.Timeout != "10m0s" || c.Rollback == nil || c.Rollback.Run != "git reset" {
		t.Fatalf("command: %+v", c)
	}
	if c.Rollback.Timeout != "" {
		t.Fatalf("zero rollback timeout must omit: %q", c.Rollback.Timeout)
	}
}

func TestToTypeJSONZeroValuesOmit(t *testing.T) {
	// a bare transition: no require, no current, no command timeout/rollback
	cfg := mtt.Type{Name: "x", Flow: mtt.Flow{
		Statuses:    []mtt.Status{{Name: "a", Kind: mtt.KindInitial}},
		Transitions: []mtt.Transition{{From: "a", To: "a", Commands: []mtt.Command{{Run: "true"}}}},
	}}
	blob := mustMarshal(t, []typeJSON{toTypeJSON(cfg, "x")})
	for _, absent := range []string{`"require"`, `"current"`, `"rollback"`, `"timeout"`, `"default"`, `"description"`} {
		if strings.Contains(blob, absent) {
			t.Fatalf("expected %s omitted, got:\n%s", absent, blob)
		}
	}
	for _, present := range []string{`"parents": []`, `"post": []`, `"commands": [`} {
		if !strings.Contains(blob, present) {
			t.Fatalf("expected %s present (non-null), got:\n%s", present, blob)
		}
	}
}
