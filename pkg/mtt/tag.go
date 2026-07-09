package mtt

import (
	"regexp"
	"strings"
)

// hashtagRe matches a #hashtag: '#' at string start or after a char that is NOT a
// Unicode letter/number, '_' or '#', capturing a token that starts and ends with a
// Unicode letter/number (interior '.', '_', '-' allowed). Deliberately \pL\pN (not
// ASCII \w) so a hashtag glued to a non-ASCII word (e.g. "тег#backend") is rejected.
var hashtagRe = regexp.MustCompile(`(?:^|[^\pL\pN_#])#([\pL\pN](?:[\pL\pN._-]*[\pL\pN])?)`)

// tagRe is the anchored canonical-tag shape (validates a whole, lowercased tag).
var tagRe = regexp.MustCompile(`^[\pL\pN](?:[\pL\pN._-]*[\pL\pN])?$`)

// NormalizeTag canonicalizes one authored tag: trim, strip one optional leading
// '#', Unicode-lowercase, validate the charset. ok is false for empty/out-of-charset
// input (the CLI turns that into a usage error). "#Auth" -> ("auth", true).
func NormalizeTag(raw string) (string, bool) {
	s := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(raw), "#"))
	if !tagRe.MatchString(s) {
		return "", false
	}
	return s, true
}

// ExtractTags returns the normalized #hashtags in text, first-seen order, deduped
// (nil when none). See hashtagRe for the token rule; results are Unicode-lowercased.
func ExtractTags(text string) []string {
	matches := hashtagRe.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		tag := strings.ToLower(m[1])
		if seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
	}
	return out
}
