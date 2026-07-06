package mtt

import "testing"

func TestCurrentActionValid(t *testing.T) {
	for _, a := range []CurrentAction{"", CurrentSet, CurrentClear} {
		if !a.Valid() {
			t.Errorf("CurrentAction(%q).Valid() = false, want true", a)
		}
	}
	for _, a := range []CurrentAction{"set ", "SET", "toggle", "clean"} {
		if a.Valid() {
			t.Errorf("CurrentAction(%q).Valid() = true, want false", a)
		}
	}
}
