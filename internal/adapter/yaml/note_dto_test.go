package yaml

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func fixedNote() mtt.Note {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return mtt.Note{Slug: "auth-design", Title: "Auth design", Tags: []string{"auth", "design"}, Body: "First body.\n", Created: ts, Updated: ts}
}

func withBody(n mtt.Note, body string) mtt.Note { n.Body = body; return n }

func withRefs(n mtt.Note, refs ...mtt.Ref) mtt.Note { n.Refs = refs; return n }

func TestMarshalParseNoteWithRefs(t *testing.T) {
	in := withRefs(fixedNote(),
		mtt.Ref{Kind: mtt.RefTask, ID: "t2"},
		mtt.Ref{Kind: mtt.RefURL, ID: "https://example.com/x", Label: "ext"},
	)
	data, err := marshalNote(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := parseNote(in.Slug, data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !reflect.DeepEqual(got.Refs, in.Refs) {
		t.Fatalf("refs round-trip: got %+v want %+v", got.Refs, in.Refs)
	}
	if got.Body != in.Body {
		t.Fatalf("body: got %q want %q", got.Body, in.Body)
	}
}

func TestMarshalNoteNoRefsUnchanged(t *testing.T) {
	// A refs-free note must not emit a refs: block (byte-identity for existing notes).
	data, err := marshalNote(fixedNote())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "refs:") {
		t.Fatalf("empty refs must be omitted, got:\n%s", data)
	}
}

func TestNoteRoundTrip(t *testing.T) {
	cases := map[string]mtt.Note{
		"full":            fixedNote(),
		"minimal":         {Slug: "stub", Created: time.Unix(10, 0).UTC(), Updated: time.Unix(10, 0).UTC()},
		"body has ---":    withBody(fixedNote(), "intro\n\n---\n\nafter a thematic break\n"),
		"no trailing nl":  withBody(fixedNote(), "no newline at end"),
		"body starts ---": withBody(fixedNote(), "---\nleading break\n"),
		"empty body":      withBody(fixedNote(), ""),
	}
	for name, in := range cases {
		data, err := marshalNote(in)
		if err != nil {
			t.Fatalf("%s: marshal: %v", name, err)
		}
		got, err := parseNote(in.Slug, data)
		if err != nil {
			t.Fatalf("%s: parse: %v", name, err)
		}
		if got.Body != in.Body {
			t.Errorf("%s: body round-trip: got %q want %q", name, got.Body, in.Body)
		}
		if got.Title != in.Title || strings.Join(got.Tags, ",") != strings.Join(in.Tags, ",") {
			t.Errorf("%s: meta round-trip: got %+v want %+v", name, got, in)
		}
		if !got.Created.Equal(in.Created) || !got.Updated.Equal(in.Updated) {
			t.Errorf("%s: time round-trip: got %v/%v want %v/%v", name, got.Created, got.Updated, in.Created, in.Updated)
		}
	}
}

func TestParseNoteCorruptNotFound(t *testing.T) {
	// A file that does not begin with "---" is corrupt — an error, and crucially NOT
	// mtt.ErrNotFound (so the store's absent-file exit-4 stays distinct).
	_, err := parseNote("x", []byte("no frontmatter here\n"))
	if err == nil {
		t.Fatal("parseNote on a headerless file: want error")
	}
	if errors.Is(err, mtt.ErrNotFound) {
		t.Fatal("corrupt-parse error must NOT be ErrNotFound")
	}
	// Unterminated frontmatter is also an error.
	if _, err := parseNote("x", []byte("---\ntitle: x\n")); err == nil {
		t.Fatal("parseNote on unterminated frontmatter: want error")
	}
}

func TestNoteGolden(t *testing.T) {
	for _, tc := range []struct {
		name string
		note mtt.Note
		file string
	}{
		{"min", mtt.Note{Slug: "stub", Created: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), Updated: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}, "note_min.md"},
		{"full", fixedNote(), "note_full.md"},
		{"refs", withRefs(fixedNote(), mtt.Ref{Kind: mtt.RefTask, ID: "t2"}, mtt.Ref{Kind: mtt.RefURL, ID: "https://example.com/x", Label: "ext"}), "note_refs.md"},
		{"priority", withPriority(fixedNote(), mtt.PriorityHigh), "note_priority.md"},
	} {
		got, err := marshalNote(tc.note)
		if err != nil {
			t.Fatalf("%s: marshal: %v", tc.name, err)
		}
		golden := filepath.Join("testdata", "golden", tc.file)
		if *update {
			if err := os.WriteFile(golden, got, 0o644); err != nil {
				t.Fatalf("write golden: %v", err)
			}
			continue
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("read golden (run -update first): %v", err)
		}
		if string(got) != string(want) {
			t.Errorf("%s serialization != golden:\n%s", tc.name, got)
		}
	}
}

func withPriority(n mtt.Note, p mtt.Priority) mtt.Note { n.Priority = p; return n }

func TestMarshalParseNotePriority(t *testing.T) {
	in := withPriority(fixedNote(), mtt.PriorityHigh)
	data, err := marshalNote(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), "priority: high") {
		t.Fatalf("priority not serialized:\n%s", data)
	}
	got, err := parseNote(in.Slug, data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Priority != mtt.PriorityHigh {
		t.Fatalf("priority round-trip: got %q", got.Priority)
	}
}

func TestMarshalNoteNoPriorityUnchanged(t *testing.T) {
	data, err := marshalNote(fixedNote())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "priority:") {
		t.Fatalf("unset priority must be omitted:\n%s", data)
	}
}

func TestParseNoteCorruptPriority(t *testing.T) {
	in := withPriority(fixedNote(), mtt.Priority("garbage"))
	data, err := marshalNote(in)
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseNote(in.Slug, data)
	if err != nil {
		t.Fatalf("corrupt priority must load, not error: %v", err)
	}
	if got.Priority != "garbage" || got.Priority.Rank() != 1 {
		t.Fatalf("corrupt priority: got %q rank=%d", got.Priority, got.Priority.Rank())
	}
}
