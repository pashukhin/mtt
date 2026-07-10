package mtt

import "testing"

func typeQueryFixture() Type {
	return Type{
		Name:    "task",
		Parents: []TypeName{"epic"},
		Flow: Flow{Statuses: []Status{
			{Name: "tbd", Kind: KindInitial},
			{Name: "doing", Kind: KindActive},
			{Name: "done", Kind: KindTerminal},
		}},
	}
}

func TestAcceptsParent(t *testing.T) {
	task := typeQueryFixture()
	if !task.AcceptsParent("epic") {
		t.Fatal("task should accept an epic parent")
	}
	if task.AcceptsParent("subtask") {
		t.Fatal("task must not accept a subtask parent")
	}
	epic := Type{Name: "epic"} // root: no parents
	if epic.AcceptsParent("epic") || epic.AcceptsParent("task") {
		t.Fatal("a root type accepts no parent")
	}
}

func TestStatusKind(t *testing.T) {
	task := typeQueryFixture()
	if k, ok := task.StatusKind("doing"); !ok || k != KindActive {
		t.Fatalf("StatusKind(doing) = %q,%v; want active,true", k, ok)
	}
	if _, ok := task.StatusKind("ghost"); ok {
		t.Fatal("unknown status must return ok=false")
	}
}

func TestStatusByName(t *testing.T) {
	typ := Type{Flow: Flow{Statuses: []Status{
		{Name: "tbd", Kind: KindInitial},
		{Name: "doing", Kind: KindActive, Description: "do the work"},
		{Name: "done", Kind: KindTerminal},
	}}}
	if s, ok := typ.StatusByName("doing"); !ok || s.Description != "do the work" {
		t.Fatalf("StatusByName(doing) = %+v,%v; want description %q, true", s, ok, "do the work")
	}
	if _, ok := typ.StatusByName("ghost"); ok {
		t.Fatal("unknown status must return ok=false")
	}
}

func TestTransitionsFrom(t *testing.T) {
	typ := Type{Flow: Flow{
		Statuses: []Status{
			{Name: "tbd", Kind: KindInitial},
			{Name: "doing", Kind: KindActive},
			{Name: "done", Kind: KindTerminal},
			{Name: "cancelled", Kind: KindTerminal},
		},
		Transitions: []Transition{
			{From: "tbd", To: "doing", Description: "start"},
			{From: "tbd", To: "cancelled"},
			{From: "doing", To: "done", Description: "finish"},
		},
	}}
	out := typ.TransitionsFrom("tbd")
	if len(out) != 2 || out[0].To != "doing" || out[1].To != "cancelled" {
		t.Fatalf("TransitionsFrom(tbd) = %+v; want doing,cancelled in definition order", out)
	}
	if got := typ.TransitionsFrom("done"); len(got) != 0 {
		t.Fatalf("TransitionsFrom(done) = %+v; want empty (terminal)", got)
	}
	if got := typ.TransitionsFrom("ghost"); len(got) != 0 {
		t.Fatalf("TransitionsFrom(ghost) = %+v; want empty (unknown)", got)
	}
}

func TestFindTransitionByName(t *testing.T) {
	typ := Type{Flow: Flow{Transitions: []Transition{
		{From: "review", To: "fix", Name: "decline"},
		{From: "review", To: "done", Name: "approve"},
		{From: "qa", To: "fix", Name: "decline"}, // same name, different source
		{From: "tbd", To: "review"},              // unnamed
	}}}
	if e, ok := typ.FindTransitionByName("review", "decline"); !ok || e.To != "fix" {
		t.Fatalf("review/decline = %+v, %v; want -> fix", e, ok)
	}
	if e, ok := typ.FindTransitionByName("qa", "decline"); !ok || e.To != "fix" {
		t.Fatalf("qa/decline = %+v, %v; want the qa-sourced edge", e, ok)
	}
	if _, ok := typ.FindTransitionByName("review", "cancel"); ok {
		t.Fatal("review/cancel must miss")
	}
	if _, ok := typ.FindTransitionByName("tbd", "decline"); ok {
		t.Fatal("tbd/decline must miss (decline is not out of tbd)")
	}
	if _, ok := typ.FindTransitionByName("tbd", ""); ok {
		t.Fatal("empty name must never match")
	}
}
