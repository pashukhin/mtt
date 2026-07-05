package yaml

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	goyaml "gopkg.in/yaml.v3"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func fixedTime() time.Time { return time.Date(2026, 7, 4, 9, 20, 0, 0, time.UTC) }

func TestTaskRoundTrip(t *testing.T) {
	want := mtt.Task{
		ID: "t1", Type: "task", Title: "fix login", Status: "tbd",
		Parent: "e1", Tags: []string{"backend", "auth"}, DependsOn: []mtt.TaskID{"t2"},
		Refs:    []mtt.Ref{{Kind: mtt.RefTask, ID: "t2", Label: "blocker"}},
		Created: fixedTime(), Updated: fixedTime(), Description: "multi\nline",
		Comments: []mtt.Comment{{ID: 1, Author: "agent", Created: fixedTime(), Body: "hi",
			Replies: []mtt.Comment{{ID: 2, Author: "human", Created: fixedTime(), Body: "yo"}}}},
		History: []mtt.HistoryEntry{{At: fixedTime(), By: "agent", Role: "impl", From: "tbd", To: "doing",
			Checks: []mtt.Check{{Cmd: "make test", Exit: 0}}}},
	}
	data, err := goyaml.Marshal(fromDomainTask(want))
	if err != nil {
		t.Fatal(err)
	}
	var yt ymlTask
	if err := goyaml.Unmarshal(data, &yt); err != nil {
		t.Fatal(err)
	}
	got, err := yt.toDomain()
	if err != nil {
		t.Fatal(err)
	}
	if !taskYAMLEqual(t, want, got) {
		t.Fatalf("round-trip mismatch:\nwant %+v\n got %+v", want, got)
	}
}

func TestTaskGoldenMinimal(t *testing.T) {
	task := mtt.Task{ID: "e1", Type: "epic", Title: "build auth", Status: "tbd",
		Created: fixedTime(), Updated: fixedTime()}
	got, err := goyaml.Marshal(fromDomainTask(task))
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "golden", "task_min.yaml")
	if *update {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run -update first): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("minimal task serialization != golden:\n%s", got)
	}
}

func taskYAMLEqual(t *testing.T, a, b mtt.Task) bool {
	t.Helper()
	da, _ := goyaml.Marshal(fromDomainTask(a))
	db, _ := goyaml.Marshal(fromDomainTask(b))
	return bytes.Equal(da, db)
}

func TestToDomainRejectsEmptyID(t *testing.T) {
	yt := ymlTask{ID: "", Type: "task", Status: "tbd", Created: "2026-07-05T00:00:00Z", Updated: "2026-07-05T00:00:00Z"}
	if _, err := yt.toDomain(); err == nil {
		t.Fatal("toDomain with empty id = nil error; want empty-id error")
	}
}

func TestToDomainRejectsEmptyType(t *testing.T) {
	yt := ymlTask{ID: "t1", Type: "", Status: "tbd", Created: "2026-07-05T00:00:00Z", Updated: "2026-07-05T00:00:00Z"}
	if _, err := yt.toDomain(); err == nil {
		t.Fatal("toDomain with empty type = nil error; want empty-type error")
	}
}

func TestToDomainRejectsEmptyStatus(t *testing.T) {
	yt := ymlTask{ID: "t1", Type: "task", Status: "", Created: "2026-07-05T00:00:00Z", Updated: "2026-07-05T00:00:00Z"}
	if _, err := yt.toDomain(); err == nil {
		t.Fatal("toDomain with empty status = nil error; want empty-status error")
	}
}

func TestTaskDTODependsOnRoundTrip(t *testing.T) {
	in := mtt.Task{
		ID: "t3", Type: "task", Title: "c", Status: "tbd",
		DependsOn: []mtt.TaskID{"t1", "t2"},
		Created:   time.Date(2026, 7, 5, 9, 0, 0, 0, time.UTC),
		Updated:   time.Date(2026, 7, 5, 9, 0, 0, 0, time.UTC),
	}
	out, err := fromDomainTask(in).toDomain()
	if err != nil {
		t.Fatal(err)
	}
	if len(out.DependsOn) != 2 || out.DependsOn[0] != "t1" || out.DependsOn[1] != "t2" {
		t.Fatalf("DependsOn round-trip = %v; want [t1 t2]", out.DependsOn)
	}
}
