package mtt

import "testing"

func TestDefaultType(t *testing.T) {
	c := Config{Types: []Type{{Name: "epic"}, {Name: "task", Default: true}}}
	got, ok := c.DefaultType()
	if !ok || got.Name != "task" {
		t.Fatalf("DefaultType = (%q,%v), want (task,true)", got.Name, ok)
	}
	// no marked default -> first type
	c2 := Config{Types: []Type{{Name: "epic"}, {Name: "task"}}}
	got2, ok2 := c2.DefaultType()
	if !ok2 || got2.Name != "epic" {
		t.Fatalf("fallback DefaultType = (%q,%v), want (epic,true)", got2.Name, ok2)
	}
	// no types -> false
	if _, ok3 := (Config{}).DefaultType(); ok3 {
		t.Fatalf("empty config DefaultType ok = true, want false")
	}
}

func TestChildrenIn(t *testing.T) {
	c := Config{Types: []Type{
		{Name: "epic"},
		{Name: "task", Parents: []string{"epic"}},
		{Name: "subtask", Parents: []string{"task"}},
	}}
	kids := c.Types[0].ChildrenIn(c)
	if len(kids) != 1 || kids[0].Name != "task" {
		t.Fatalf("ChildrenIn(epic) = %v, want [task]", names(kids))
	}
	if k := c.Types[2].ChildrenIn(c); len(k) != 0 {
		t.Fatalf("ChildrenIn(subtask) = %v, want []", names(k))
	}
}

func names(ts []Type) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Name
	}
	return out
}
