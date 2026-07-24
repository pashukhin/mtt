package mtt

import (
	"reflect"
	"testing"
)

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
		{Name: "task", Parents: []TypeName{"epic"}},
		{Name: "subtask", Parents: []TypeName{"task"}},
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
		out[i] = string(t.Name)
	}
	return out
}

func TestIsRoot(t *testing.T) {
	if !(Type{}).IsRoot() {
		t.Fatal("no parents => root")
	}
	if (Type{Parents: []TypeName{"epic"}}).IsRoot() {
		t.Fatal("with parents => not root")
	}
}

func TestInitialStatus(t *testing.T) {
	// first initial wins when none is marked default
	ty := Type{Flow: Flow{Statuses: []Status{
		{Name: "a", Kind: KindInitial},
		{Name: "b", Kind: KindInitial},
		{Name: "c", Kind: KindActive},
	}}}
	if s, ok := ty.InitialStatus(); !ok || s.Name != "a" {
		t.Fatalf("first initial: got %q ok=%v", s.Name, ok)
	}
	// a marked default initial wins over order
	ty.Statuses[1].Default = true
	if s, ok := ty.InitialStatus(); !ok || s.Name != "b" {
		t.Fatalf("default initial: got %q ok=%v", s.Name, ok)
	}
	// no initial => false
	none := Type{Flow: Flow{Statuses: []Status{{Name: "x", Kind: KindTerminal}}}}
	if _, ok := none.InitialStatus(); ok {
		t.Fatal("no initial should return false")
	}
}

func TestTypeByName(t *testing.T) {
	c := Config{Types: []Type{{Name: "epic"}, {Name: "task"}}}
	if ty, ok := c.TypeByName("task"); !ok || ty.Name != "task" {
		t.Fatalf("lookup task: %q ok=%v", ty.Name, ok)
	}
	if _, ok := c.TypeByName("ghost"); ok {
		t.Fatal("ghost should not resolve")
	}
}

func TestFindTransition(t *testing.T) {
	ty := Type{Flow: Flow{Transitions: []Transition{
		{From: "tbd", To: "wip", Current: CurrentSet},
		{From: "wip", To: "done", Current: CurrentClear},
	}}}
	e, ok := ty.FindTransition("tbd", "wip")
	if !ok || e.Current != CurrentSet {
		t.Fatalf("FindTransition(tbd,wip) = (%+v,%v), want the set edge", e, ok)
	}
	if _, ok := ty.FindTransition("tbd", "done"); ok {
		t.Fatal("FindTransition(tbd,done) = ok, want false (no such edge)")
	}
}

func TestEffectivePost(t *testing.T) {
	d1 := Command{Run: "default-1"}
	d2 := Command{Run: "default-2"}
	own := Command{Run: "own"}
	typ := Type{Name: "task", PostDefaults: []Command{d1, d2}}
	cases := []struct {
		name string
		tr   Transition
		want []string
	}{
		{"defaults prepend to edge post", Transition{Post: []Command{own}}, []string{"default-1", "default-2", "own"}},
		{"defaults alone when edge has no post", Transition{}, []string{"default-1", "default-2"}},
		{"opt-out yields edge post only", Transition{Post: []Command{own}, SkipPostDefaults: true}, []string{"own"}},
		{"opt-out with empty post yields none", Transition{SkipPostDefaults: true}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := typ.EffectivePost(tc.tr)
			var runs []string
			for _, c := range got {
				runs = append(runs, c.Run)
			}
			if !reflect.DeepEqual(runs, tc.want) {
				t.Fatalf("EffectivePost runs = %v, want %v", runs, tc.want)
			}
		})
	}
	// no defaults: the edge's own slice must come back unchanged (the common
	// case must not allocate a copy).
	bare := Type{Name: "bare"}
	if got := bare.EffectivePost(Transition{Post: []Command{own}}); len(got) != 1 || got[0].Run != "own" {
		t.Fatalf("bare EffectivePost = %v", got)
	}
}
