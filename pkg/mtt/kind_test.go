package mtt

import "testing"

func TestStatusKindValid(t *testing.T) {
	for _, k := range []StatusKind{KindInitial, KindActive, KindTerminal} {
		if !k.Valid() {
			t.Errorf("Valid(%q) = false, want true", k)
		}
	}
	for _, k := range []StatusKind{"", "todo", "Initial", "done"} {
		if k.Valid() {
			t.Errorf("Valid(%q) = true, want false", k)
		}
	}
}

func TestStatusKindConstants(t *testing.T) {
	if KindInitial != "initial" || KindActive != "active" || KindTerminal != "terminal" {
		t.Fatalf("kind constants drifted: %q %q %q", KindInitial, KindActive, KindTerminal)
	}
}
