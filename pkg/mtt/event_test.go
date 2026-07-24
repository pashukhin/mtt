package mtt

import "testing"

func TestEventKindValid(t *testing.T) {
	for _, k := range []EventKind{EventCreate, EventUpdate, EventDelete} {
		if !k.Valid() {
			t.Fatalf("%q must be valid", k)
		}
	}
	for _, k := range []EventKind{"", "created", "CREATE"} {
		if k.Valid() {
			t.Fatalf("%q must be invalid", k)
		}
	}
}

func TestEventHooksHook(t *testing.T) {
	h := EventHooks{
		Create: EventHook{Post: []Command{{Run: "c"}}},
		Update: EventHook{Post: []Command{{Run: "u"}}},
		Delete: EventHook{Post: []Command{{Run: "d"}}},
	}
	for kind, want := range map[EventKind]string{EventCreate: "c", EventUpdate: "u", EventDelete: "d"} {
		if got := h.Hook(kind); len(got.Post) != 1 || got.Post[0].Run != want {
			t.Fatalf("Hook(%q) = %+v, want run %q", kind, got, want)
		}
	}
	if got := h.Hook("bogus"); len(got.Post) != 0 {
		t.Fatalf("Hook(bogus) = %+v, want zero", got)
	}
}
