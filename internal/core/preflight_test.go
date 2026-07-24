package core

import (
	"errors"
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// TestEditorsNoRunPreflight pins the universal §6 rule: EVERY mutating method
// validates the --no-run attribution BEFORE any load or persist.
func TestEditorsNoRunPreflight(t *testing.T) {
	bare := EventOptions{NoRun: true} // no By/Why → must be refused
	cases := []struct {
		name string
		call func(store mtt.TaskStore) error
	}{
		{"tag add", func(s mtt.TaskStore) error {
			_, _, err := NewTagEditor(s, testClock, nil).AddTags("t1", []string{"x"}, bare)
			return err
		}},
		{"tag rm", func(s mtt.TaskStore) error {
			_, _, err := NewTagEditor(s, testClock, nil).RemoveTags("t1", []string{"x"}, bare)
			return err
		}},
		{"dep add", func(s mtt.TaskStore) error {
			_, err := NewDependencyEditor(s, testClock, nil).AddDependency("t1", "t2", bare)
			return err
		}},
		{"dep rm", func(s mtt.TaskStore) error {
			_, err := NewDependencyEditor(s, testClock, nil).RemoveDependency("t1", "t2", bare)
			return err
		}},
		{"ref add", func(s mtt.TaskStore) error {
			_, err := NewRefEditor(s, testClock, nil).AddRef("t1", mtt.Ref{Kind: mtt.RefURL, ID: "https://x"}, false, bare)
			return err
		}},
		{"ref rm", func(s mtt.TaskStore) error {
			_, err := NewRefEditor(s, testClock, nil).RemoveRef("t1", mtt.RefURL, "https://x", bare)
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newMemStore(tbdTask("t1"), tbdTask("t2"))
			err := tc.call(store)
			if !errors.Is(err, ErrMissingAttribution) {
				t.Fatalf("want ErrMissingAttribution, got %v", err)
			}
			if got, _ := store.Get("t1"); got.Updated != testClock() {
				t.Fatal("store mutated despite failed preflight")
			}
		})
	}
}
