package yaml

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SourceKind is how a `mtt init --template <arg>` value resolves.
type SourceKind int

// The template source kinds a --template arg resolves to.
const (
	SourceBuiltin SourceKind = iota // a compiled-in template name (default)
	SourceFile                      // a local file path
	SourceURL                       // an https URL
)

// ClassifyTemplate resolves the source kind of a --template arg (pure): https://
// → URL (http:// is a hard error); a path shape (separator, or .yaml/.yml suffix)
// → file; otherwise a bare token → builtin name (name lookup errors later).
func ClassifyTemplate(arg string) (SourceKind, string, error) {
	switch {
	case strings.HasPrefix(arg, "https://"):
		return SourceURL, arg, nil
	case strings.HasPrefix(arg, "http://"):
		return 0, "", fmt.Errorf("remote templates must be https (got %q)", arg)
	case strings.ContainsRune(arg, '/') || strings.ContainsRune(arg, filepath.Separator) ||
		strings.HasSuffix(arg, ".yaml") || strings.HasSuffix(arg, ".yml"):
		return SourceFile, arg, nil
	default:
		return SourceBuiltin, arg, nil
	}
}
