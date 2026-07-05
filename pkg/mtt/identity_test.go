package mtt

import "testing"

func TestNewTaskID(t *testing.T) {
	got, err := NewTaskID("e1")
	if err != nil || got != TaskID("e1") {
		t.Fatalf("NewTaskID(\"e1\") = %q, %v; want e1, nil", got, err)
	}
	if _, err := NewTaskID(""); err == nil {
		t.Fatal("NewTaskID(\"\") = nil error; want empty-id error")
	}
}

func TestNewTypeName(t *testing.T) {
	got, err := NewTypeName("task")
	if err != nil || got != TypeName("task") {
		t.Fatalf("NewTypeName(\"task\") = %q, %v; want task, nil", got, err)
	}
	if _, err := NewTypeName(""); err == nil {
		t.Fatal("NewTypeName(\"\") = nil error; want empty-name error")
	}
}

func TestNewStatusName(t *testing.T) {
	got, err := NewStatusName("tbd")
	if err != nil || got != StatusName("tbd") {
		t.Fatalf("NewStatusName(\"tbd\") = %q, %v; want tbd, nil", got, err)
	}
	if _, err := NewStatusName(""); err == nil {
		t.Fatal("NewStatusName(\"\") = nil error; want empty-name error")
	}
}
