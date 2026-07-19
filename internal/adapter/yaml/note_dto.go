package yaml

import (
	"bytes"
	"fmt"
	"time"

	goyaml "gopkg.in/yaml.v3"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// ymlNote is the YAML frontmatter DTO for a note. A struct (not a map) so field
// order is deterministic. The slug is NOT here — it is the file name (single source
// of truth for identity). created/updated are always present (frontmatter is never
// empty), which is what makes the "first closing ---" read rule unambiguous.
type ymlNote struct {
	Title   string   `yaml:"title,omitempty"`
	Tags    []string `yaml:"tags,omitempty"`
	Created string   `yaml:"created"`
	Updated string   `yaml:"updated"`
}

// noteDelim is the frontmatter delimiter line (with its newline).
const noteDelim = "---\n"

// marshalNote serializes a note to the on-disk hybrid document: "---\n" +
// struct-ordered YAML frontmatter + "---\n" + the body VERBATIM (its bytes, incl.
// any "---" lines and its trailing-newline state, are preserved exactly).
func marshalNote(n mtt.Note) ([]byte, error) {
	fm, err := goyaml.Marshal(ymlNote{
		Title:   n.Title,
		Tags:    n.Tags,
		Created: n.Created.UTC().Format(time.RFC3339),
		Updated: n.Updated.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal note %s: %w", n.Slug, err)
	}
	var b bytes.Buffer
	b.WriteString(noteDelim)
	b.Write(fm)
	b.WriteString(noteDelim)
	b.WriteString(n.Body)
	return b.Bytes(), nil
}

// parseNote splits a note file into frontmatter + body and maps to the domain. The
// file MUST begin with "---\n"; the frontmatter runs up to the FIRST subsequent line
// that is exactly "---"; the body is everything after that delimiter, byte-for-byte
// (so "---" inside the body is preserved — only the first closing delimiter counts).
// Only the frontmatter bytes are unmarshaled — never the whole file ("\n---\n" is
// yaml's document separator). slug is the (already-validated) file name.
func parseNote(slug mtt.NoteSlug, data []byte) (mtt.Note, error) {
	if !bytes.HasPrefix(data, []byte(noteDelim)) {
		return mtt.Note{}, fmt.Errorf("note %s: missing frontmatter (no leading ---)", slug)
	}
	rest := data[len(noteDelim):]
	idx := bytes.Index(rest, []byte("\n"+noteDelim)) // the closing delimiter line
	if idx < 0 {
		return mtt.Note{}, fmt.Errorf("note %s: unterminated frontmatter (no closing ---)", slug)
	}
	fmBytes := rest[:idx+1] // include the last frontmatter line's trailing newline
	body := rest[idx+1+len(noteDelim):]
	var yn ymlNote
	if err := goyaml.Unmarshal(fmBytes, &yn); err != nil {
		return mtt.Note{}, fmt.Errorf("note %s: parse frontmatter: %w", slug, err)
	}
	created, err := time.Parse(time.RFC3339, yn.Created)
	if err != nil {
		return mtt.Note{}, fmt.Errorf("note %s: created: %w", slug, err)
	}
	updated, err := time.Parse(time.RFC3339, yn.Updated)
	if err != nil {
		return mtt.Note{}, fmt.Errorf("note %s: updated: %w", slug, err)
	}
	return mtt.Note{Slug: slug, Title: yn.Title, Tags: yn.Tags, Body: string(body), Created: created, Updated: updated}, nil
}
