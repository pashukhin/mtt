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
