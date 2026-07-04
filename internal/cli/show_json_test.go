package cli

import (
	"encoding/json"
	"testing"
)

func TestShowJSON(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := runRoot(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runOut(t, "add", "--type", "epic", "build auth"); err != nil {
		t.Fatal(err)
	}
	out, _, err := runOut(t, "show", "--json", "e1")
	if err != nil {
		t.Fatalf("show --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out)
	}
	if got["id"] != "e1" || got["type"] != "epic" || got["title"] != "build auth" {
		t.Fatalf("json fields = %v", got)
	}
	if got["status"] != "tbd" {
		t.Fatalf("status = %v, want tbd", got["status"])
	}
}
