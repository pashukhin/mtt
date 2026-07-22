package cli

import (
	"strings"
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestToTagsValid(t *testing.T) {
	got, err := toTags([]string{"#Auth", "Backend"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "auth" || got[1] != "backend" {
		t.Fatalf("toTags = %v; want [auth backend]", got)
	}
}

func TestToTagsInvalid(t *testing.T) {
	if _, err := toTags([]string{"bad tag"}); err == nil || !strings.Contains(err.Error(), "invalid tag") {
		t.Fatalf("want invalid-tag error, got %v", err)
	}
}

func TestFormatTaskShowsTags(t *testing.T) {
	task := mtt.Task{ID: "t1", Type: "task", Title: "a", Status: "tbd", Tags: []string{"auth", "urgent"}}
	out := formatTask(task, nil, nil, nil, "", nil)
	if !strings.Contains(out, "tags:     auth, urgent") {
		t.Fatalf("formatTask missing tags line:\n%s", out)
	}
}

func TestTaskJSONCarriesTags(t *testing.T) {
	v := toTaskJSON(mtt.Task{ID: "t1", Type: "task", Status: "tbd", Tags: []string{"auth"}})
	if len(v.Tags) != 1 || v.Tags[0] != "auth" {
		t.Fatalf("taskJSON.Tags = %v; want [auth]", v.Tags)
	}
}
