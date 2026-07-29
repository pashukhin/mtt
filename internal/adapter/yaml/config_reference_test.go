package yaml

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// TestConfigReferenceCoversSchema fails if a config field declared in dto.go is
// not documented (as a backticked span `field`, optionally `field:`) in
// CONFIG_REFERENCE.md. The drift guard so the reference can never silently go
// stale-incomplete — the sin that created t83. Guards the field ENUMERATION;
// the validity SEMANTICS are a review concern (see the doc + FLOW_GUIDE §11).
func TestConfigReferenceCoversSchema(t *testing.T) {
	dto, err := os.ReadFile("dto.go") // test CWD is the package dir
	if err != nil {
		t.Fatalf("read dto.go: %v", err)
	}
	root, err := FindRoot(".")
	if err != nil {
		t.Fatalf("FindRoot: %v", err)
	}
	ref, err := os.ReadFile(filepath.Join(root, "CONFIG_REFERENCE.md"))
	if err != nil {
		t.Fatalf("read CONFIG_REFERENCE.md: %v", err)
	}
	doc := string(ref)

	tag := regexp.MustCompile(`yaml:"([^",]+)`) // name = token before the first quote/comma
	seen := map[string]bool{}
	for _, m := range tag.FindAllStringSubmatch(string(dto), -1) {
		name := m[1]
		if name == "" || name == "-" || seen[name] {
			continue
		}
		seen[name] = true
		// backtick-bounded so a common word (name/default/from/to/post/run/...) must be a
		// DELIBERATE documented field, and `post` never satisfies `post_defaults`.
		if !regexp.MustCompile("`" + regexp.QuoteMeta(name) + ":?`").MatchString(doc) {
			t.Errorf("config field %q is in dto.go but not documented (as `%s`) in CONFIG_REFERENCE.md", name, name)
		}
	}
	if len(seen) == 0 {
		t.Fatal("no yaml tags found in dto.go — the extractor is broken")
	}
}
