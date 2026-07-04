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
		Parent: "e1", Tags: []string{"backend", "auth"}, DependsOn: []string{"t2"},
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
