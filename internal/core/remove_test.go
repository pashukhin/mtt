package core

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// deleted reports whether id is absent from the store (memStore.Delete removes it).
func (m *memStore) deleted(id mtt.TaskID) bool { _, ok := m.byID[id]; return !ok }

// fakeAudit is the AuditStore test double; failOnID makes Append error for one id.
type fakeAudit struct {
	entries  []mtt.AuditEntry
	failOnID mtt.TaskID
}

func (f *fakeAudit) Append(e mtt.AuditEntry) error {
	if e.TaskID == f.failOnID {
		return fmt.Errorf("disk full")
	}
	f.entries = append(f.entries, e)
	return nil
}

// remover wires a Remover with a throwaway audit + the fixed test clock, for the
// tests that don't inspect the audit.
func remover(store mtt.TaskStore) *Remover { return NewRemover(store, &fakeAudit{}, testClock, nil) }

func tbdTask(id mtt.TaskID) mtt.Task {
	return mtt.Task{ID: id, Type: "task", Status: "tbd", Created: testClock(), Updated: testClock()}
}

func TestRemoveUnreferenced(t *testing.T) {
	m := newMemStore(mtt.Task{ID: "t1", Type: "task", Status: "tbd"})
	if err := remover(m).Remove("t1", false, "", "", nil, false); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, ok := m.byID["t1"]; ok {
		t.Fatal("task not deleted from store")
	}
}

func TestRemoveNotFound(t *testing.T) {
	m := newMemStore()
	err := remover(m).Remove("t99", false, "", "", nil, false)
	if !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("err = %v; want ErrNotFound", err)
	}
}

func TestRemoveRejectedByDependent(t *testing.T) {
	// t2 depends on t1; removing t1 without --force is rejected and t1 survives.
	m := newMemStore(
		mtt.Task{ID: "t1", Type: "task", Status: "tbd"},
		mtt.Task{ID: "t2", Type: "task", Status: "tbd", DependsOn: []mtt.TaskID{"t1"}},
	)
	err := remover(m).Remove("t1", false, "", "", nil, false)
	if err == nil || !strings.Contains(err.Error(), "t2") {
		t.Fatalf("err = %v; want a referenced-by-t2 error", err)
	}
	if _, ok := m.byID["t1"]; !ok {
		t.Fatal("t1 must NOT be deleted on a rejected remove")
	}
}

func TestRemoveRejectedByChild(t *testing.T) {
	// s1's parent is t1; removing t1 without --force is rejected.
	m := newMemStore(
		mtt.Task{ID: "t1", Type: "task", Status: "tbd"},
		mtt.Task{ID: "s1", Type: "subtask", Status: "tbd", Parent: "t1"},
	)
	err := remover(m).Remove("t1", false, "", "", nil, false)
	if err == nil || !strings.Contains(err.Error(), "s1") {
		t.Fatalf("err = %v; want a referenced-by-s1 error", err)
	}
}

func TestRemoveForceDeletesReferenced(t *testing.T) {
	m := newMemStore(
		mtt.Task{ID: "t1", Type: "task", Status: "tbd"},
		mtt.Task{ID: "t2", Type: "task", Status: "tbd", DependsOn: []mtt.TaskID{"t1"}},
	)
	if err := remover(m).Remove("t1", true, "alice", "cleanup", nil, false); err != nil {
		t.Fatalf("force remove: %v", err)
	}
	if _, ok := m.byID["t1"]; ok {
		t.Fatal("t1 not deleted under --force")
	}
}

func TestRemoveReferencedDedup(t *testing.T) {
	// t2 is BOTH a child and a dependent of t1 → must appear once in the message.
	m := newMemStore(
		mtt.Task{ID: "t1", Type: "task", Status: "tbd"},
		mtt.Task{ID: "t2", Type: "task", Status: "tbd", Parent: "t1", DependsOn: []mtt.TaskID{"t1"}},
	)
	err := remover(m).Remove("t1", false, "", "", nil, false)
	if err == nil || strings.Count(err.Error(), "t2") != 1 {
		t.Fatalf("err = %v; want t2 mentioned exactly once", err)
	}
}

func TestRemoveManySubgraphIgnore(t *testing.T) {
	// e1 has child t1; deleting BOTH in one call ignores the in-set reference.
	m := newMemStore(
		mtt.Task{ID: "e1", Type: "epic", Status: "tbd"},
		mtt.Task{ID: "t1", Type: "task", Status: "tbd", Parent: "e1"},
	)
	res, err := remover(m).RemoveMany([]mtt.TaskID{"e1", "t1"}, false, "", "", nil, false)
	if err != nil {
		t.Fatalf("pre-flight: %v", err)
	}
	if len(res) != 2 || res[0].Err != nil || res[1].Err != nil {
		t.Fatalf("results = %+v; want both nil", res)
	}
	if len(m.byID) != 0 {
		t.Fatalf("store not empty: %v", m.byID)
	}
}

func TestRemoveManyExternalRejects(t *testing.T) {
	// deleting only e1 (child t1 NOT in the set) is rejected without --force.
	m := newMemStore(
		mtt.Task{ID: "e1", Type: "epic", Status: "tbd"},
		mtt.Task{ID: "t1", Type: "task", Status: "tbd", Parent: "e1"},
	)
	res, err := remover(m).RemoveMany([]mtt.TaskID{"e1"}, false, "", "", nil, false)
	if err != nil {
		t.Fatalf("pre-flight: %v", err)
	}
	if len(res) != 1 || res[0].Err == nil || !strings.Contains(res[0].Err.Error(), "t1") {
		t.Fatalf("res = %+v; want referenced-by-t1", res)
	}
	if _, ok := m.byID["e1"]; !ok {
		t.Fatal("e1 must survive a rejected delete")
	}
}

func TestRemoveManyForceOverrides(t *testing.T) {
	m := newMemStore(
		mtt.Task{ID: "e1", Type: "epic", Status: "tbd"},
		mtt.Task{ID: "t1", Type: "task", Status: "tbd", Parent: "e1"},
	)
	res, err := remover(m).RemoveMany([]mtt.TaskID{"e1"}, true, "alice", "cleanup", nil, false)
	if err != nil {
		t.Fatalf("pre-flight: %v", err)
	}
	if res[0].Err != nil {
		t.Fatalf("force err: %v", res[0].Err)
	}
	if _, ok := m.byID["e1"]; ok {
		t.Fatal("e1 not deleted under force")
	}
}

func TestRemoveManyBestEffort(t *testing.T) {
	// a missing id does not stop the rest; each has its own result.
	m := newMemStore(mtt.Task{ID: "t1", Type: "task", Status: "tbd"})
	res, err := remover(m).RemoveMany([]mtt.TaskID{"t1", "t99"}, false, "", "", nil, false)
	if err != nil {
		t.Fatalf("pre-flight: %v", err)
	}
	if len(res) != 2 || res[0].Err != nil {
		t.Fatalf("t1 should succeed: %+v", res)
	}
	if !errors.Is(res[1].Err, mtt.ErrNotFound) {
		t.Fatalf("t99 err = %v; want ErrNotFound", res[1].Err)
	}
	if _, ok := m.byID["t1"]; ok {
		t.Fatal("t1 not deleted")
	}
}

func TestRemoveManyDedup(t *testing.T) {
	m := newMemStore(mtt.Task{ID: "t1", Type: "task", Status: "tbd"})
	res, err := remover(m).RemoveMany([]mtt.TaskID{"t1", "t1"}, false, "", "", nil, false)
	if err != nil {
		t.Fatalf("pre-flight: %v", err)
	}
	if len(res) != 1 || res[0].Err != nil {
		t.Fatalf("res = %+v; want a single ok", res)
	}
}

// --- t5: force ⇒ who+why (pre-flight) + append-before-delete audit ---

func TestRemoveMany_ForceRequiresWhoAndWhy(t *testing.T) {
	store := newMemStore(tbdTask("t1"))
	audit := &fakeAudit{}
	res, err := NewRemover(store, audit, testClock, nil).RemoveMany([]mtt.TaskID{"t1"}, true, "", "", nil, false)
	if !errors.Is(err, ErrMissingAttribution) {
		t.Fatalf("want pre-flight ErrMissingAttribution, got %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("want empty results on pre-flight fail, got %d", len(res))
	}
	if store.deleted("t1") {
		t.Fatal("nothing must be deleted on pre-flight failure")
	}
	if len(audit.entries) != 0 {
		t.Fatal("no audit entry on pre-flight failure")
	}
}

func TestRemoveMany_ForceAppendsBeforeDelete(t *testing.T) {
	store := newMemStore(tbdTask("t1"))
	audit := &fakeAudit{}
	res, err := NewRemover(store, audit, testClock, nil).RemoveMany([]mtt.TaskID{"t1"}, true, "alice", "cleanup", nil, false)
	if err != nil {
		t.Fatalf("pre-flight error: %v", err)
	}
	if res[0].Err != nil {
		t.Fatalf("delete error: %v", res[0].Err)
	}
	if len(audit.entries) != 1 || audit.entries[0].TaskID != "t1" ||
		audit.entries[0].Who != "alice" || audit.entries[0].Why != "cleanup" || audit.entries[0].Action != "rm --force" {
		t.Fatalf("audit entry wrong: %+v", audit.entries)
	}
	if !store.deleted("t1") {
		t.Fatal("task should be deleted after successful append")
	}
}

func TestRemoveMany_AppendFailureSkipsDelete(t *testing.T) {
	store := newMemStore(tbdTask("t1"), tbdTask("t2"))
	audit := &fakeAudit{failOnID: "t1"}
	res, err := NewRemover(store, audit, testClock, nil).RemoveMany([]mtt.TaskID{"t1", "t2"}, true, "alice", "cleanup", nil, false)
	if err != nil {
		t.Fatalf("pre-flight error: %v", err)
	}
	if res[0].Err == nil {
		t.Fatal("t1 append failed → its RemoveResult.Err must be set")
	}
	if store.deleted("t1") {
		t.Fatal("t1 must NOT be deleted when its audit append failed")
	}
	if res[1].Err != nil || !store.deleted("t2") {
		t.Fatalf("t2 should proceed independently: err=%v deleted=%v", res[1].Err, store.deleted("t2"))
	}
}

func TestRemoveMany_NoForceUnchanged(t *testing.T) {
	store := newMemStore(tbdTask("t1"))
	audit := &fakeAudit{}
	res, err := NewRemover(store, audit, testClock, nil).RemoveMany([]mtt.TaskID{"t1"}, false, "", "", nil, false)
	if err != nil {
		t.Fatalf("no-force must not pre-flight error: %v", err)
	}
	if res[0].Err != nil || !store.deleted("t1") {
		t.Fatalf("no-force delete: err=%v deleted=%v", res[0].Err, store.deleted("t1"))
	}
	if len(audit.entries) != 0 {
		t.Fatal("no audit on non-force delete")
	}
}

func TestRemoverRefGuardCrossStore(t *testing.T) {
	// t5 exists; t2 (task) and note "a" both reference t5 via a ref.
	tasks := []mtt.Task{
		{ID: "t5"},
		{ID: "t2", Refs: []mtt.Ref{{Kind: mtt.RefTask, ID: "t5"}}},
	}
	notes := []mtt.Note{{Slug: "a", Refs: []mtt.Ref{{Kind: mtt.RefTask, ID: "t5"}}}}
	store := newMemStore(tasks...)
	bl := NewBacklinks(tasks, notes)
	r := NewRemover(store, &fakeAudit{}, testClock, nil)

	// refuse: referenced by a task AND a note
	if err := r.Remove("t5", false, "", "", bl, false); err == nil {
		t.Fatal("must refuse: t5 referenced by t2 and note a")
	}
	// force deletes (leaves dangling)
	if err := r.Remove("t5", true, "me", "why", bl, false); err != nil {
		t.Fatalf("force: %v", err)
	}
}

// orderRunner records interleaving markers into a shared log (delete-vs-run
// adjacency proof for per-entity firing inside the RemoveMany loop).
type orderRunner struct {
	fakeRunner
	log *[]string
}

func (o *orderRunner) Run(commands []mtt.Command) ([]mtt.Check, error) {
	for _, c := range commands {
		*o.log = append(*o.log, "run:"+c.Run)
	}
	return o.fakeRunner.Run(commands)
}

// orderStore mirrors the markers on Delete.
type orderStore struct {
	*memStore
	log *[]string
}

func (o *orderStore) Delete(id mtt.TaskID) error {
	*o.log = append(*o.log, "delete:"+string(id))
	return o.memStore.Delete(id)
}

func TestRemoveManyFiresDeletePerEntityInsideLoop(t *testing.T) {
	var log []string
	store := &orderStore{memStore: newMemStore(tbdTask("t1"), tbdTask("t2")), log: &log}
	runner := &orderRunner{log: &log}
	cfg := eventCfg(taskHook(mtt.EventDelete, "{{.ID}}"))
	r := NewRemover(store, &fakeAudit{}, testClock, NewEventEmitter(cfg, runner, &fakeAudit{}, testClock))
	res, err := r.RemoveMany([]mtt.TaskID{"t1", "t2"}, true, "me", "why", Backlinks{}, false)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	for _, rr := range res {
		if rr.Err != nil {
			t.Fatalf("%s: %v", rr.ID, rr.Err)
		}
	}
	want := []string{"delete:t1", "run:t1", "delete:t2", "run:t2"}
	if len(log) != 4 || log[0] != want[0] || log[1] != want[1] || log[2] != want[2] || log[3] != want[3] {
		t.Fatalf("order = %v, want %v (fire per entity, inside the loop)", log, want)
	}
}

func TestRemoveManyEventFailureRidesResult(t *testing.T) {
	cfg := eventCfg(taskHook(mtt.EventDelete, "boom-{{.ID}}"))
	store := newMemStore(tbdTask("t1"), tbdTask("t2"))
	runner := &fakeRunner{failSubstr: "boom-t1"}
	r := NewRemover(store, &fakeAudit{}, testClock, NewEventEmitter(cfg, runner, &fakeAudit{}, testClock))
	res, err := r.RemoveMany([]mtt.TaskID{"t1", "t2"}, true, "me", "why", Backlinks{}, false)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	var pe *PostActionError
	if !errors.As(res[0].Err, &pe) {
		t.Fatalf("t1 result = %v, want *PostActionError", res[0].Err)
	}
	if res[1].Err != nil {
		t.Fatalf("t2 must delete cleanly, got %v", res[1].Err)
	}
	if !store.deleted("t1") || !store.deleted("t2") {
		t.Fatal("both tasks must be deleted (best-effort, mutation kept)")
	}
}

func TestRemoveForceNoRunWritesOneRecord(t *testing.T) {
	audit := &fakeAudit{}
	runner := &fakeRunner{}
	cfg := eventCfg(taskHook(mtt.EventDelete, "echo hi"))
	r := NewRemover(newMemStore(tbdTask("t1")), audit, testClock, NewEventEmitter(cfg, runner, audit, testClock))
	res, err := r.RemoveMany([]mtt.TaskID{"t1"}, true, "me", "sign", Backlinks{}, true)
	if err != nil || res[0].Err != nil {
		t.Fatalf("unexpected: %v / %v", err, res)
	}
	if runner.called {
		t.Fatal("--no-run must skip the pipeline")
	}
	if len(audit.entries) != 1 {
		t.Fatalf("want exactly ONE record (pin iii), got %+v", audit.entries)
	}
	if got := audit.entries[0]; got.Action != "rm --force --no-run" || got.TaskID != "t1" {
		t.Fatalf("record = %+v", got)
	}
}

func TestRemoveNoRunPreflight(t *testing.T) {
	store := newMemStore(tbdTask("t1"))
	r := NewRemover(store, &fakeAudit{}, testClock, nil)
	_, err := r.RemoveMany([]mtt.TaskID{"t1"}, false, "", "", Backlinks{}, true)
	if !errors.Is(err, ErrMissingAttribution) {
		t.Fatalf("want ErrMissingAttribution, got %v", err)
	}
	if store.deleted("t1") {
		t.Fatal("preflight must run BEFORE any deletion")
	}
}

func TestRemoveNoRunSkipRecord(t *testing.T) {
	audit := &fakeAudit{}
	cfg := eventCfg(taskHook(mtt.EventDelete, "echo hi"))
	runner := &fakeRunner{}
	r := NewRemover(newMemStore(tbdTask("t1")), audit, testClock, NewEventEmitter(cfg, runner, audit, testClock))
	res, err := r.RemoveMany([]mtt.TaskID{"t1"}, false, "me", "sign", Backlinks{}, true)
	if err != nil || res[0].Err != nil {
		t.Fatalf("unexpected: %v / %v", err, res)
	}
	if runner.called {
		t.Fatal("--no-run must skip the pipeline")
	}
	if len(audit.entries) != 1 || audit.entries[0].Action != "rm --no-run" {
		t.Fatalf("skip record = %+v", audit.entries)
	}
}
