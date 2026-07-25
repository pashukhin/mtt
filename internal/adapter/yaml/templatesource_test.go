package yaml

import "testing"

func TestClassifyTemplate(t *testing.T) {
	cases := []struct {
		arg  string
		kind SourceKind
		err  bool
	}{
		{"default", SourceBuiltin, false},
		{"coding", SourceBuiltin, false}, // bare unknown name is still "builtin" (name-resolution errors later)
		{"x.yaml", SourceFile, false},
		{"./x.yml", SourceFile, false},
		{"a/b/c", SourceFile, false},
		{"https://h/x.yaml", SourceURL, false},
		{"http://h/x.yaml", 0, true}, // https only
	}
	for _, c := range cases {
		k, _, err := ClassifyTemplate(c.arg)
		if (err != nil) != c.err {
			t.Fatalf("%q: err=%v want err=%v", c.arg, err, c.err)
		}
		if err == nil && k != c.kind {
			t.Fatalf("%q: kind=%d want %d", c.arg, k, c.kind)
		}
	}
}
