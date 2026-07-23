package mtt

import (
	"regexp"
	"strings"
)

// hashtagRe matches a #hashtag: '#' at string start or after a char that is NOT a
// Unicode letter/number, '_' or '#', capturing a token that starts with a Unicode
// letter/number and ends with a letter/number or combining mark (interior letters/
// numbers/marks plus '.', '_', '-'). Deliberately \pL\pN (not ASCII \w) so a hashtag
// glued to a non-ASCII word (e.g. "тег#backend") is rejected; \pM keeps a combining
// mark from truncating the token (a decomposed "áuth"/"йога" stays whole, not "a"/"и").
var hashtagRe = regexp.MustCompile(`(?:^|[^\pL\pN_#])#([\pL\pN](?:[\pL\pN\pM._-]*[\pL\pN\pM])?)`)

// tagRe is the anchored canonical-tag shape (validates a whole, lowercased tag).
var tagRe = regexp.MustCompile(`^[\pL\pN](?:[\pL\pN\pM._-]*[\pL\pN\pM])?$`)

// urlRe matches a scheme://non-space run — a pasted link. Its trailing #fragment
// (e.g. .../page#section) would otherwise mint a bogus tag, so ExtractTags skips
// any hashtag whose token falls inside such a run. Only scheme:// is treated as a
// URL (a schemeless "url-ish" string is still scanned — a deliberate boundary).
var urlRe = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://\S+`)

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
// A #fragment inside a scheme:// URL run is skipped, so a pasted link mints no tags
// (a schemeless "url-ish" string is still scanned — see urlRe).
func ExtractTags(text string) []string {
	matches := hashtagRe.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return nil
	}
	urls := urlRe.FindAllStringIndex(text, -1)
	seen := map[string]bool{}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		// m = [matchStart, matchEnd, tokenStart, tokenEnd]; the tag token is
		// group 1. Skip a token that sits inside a URL run (a link fragment).
		if indexInSpans(m[2], urls) {
			continue
		}
		tag := strings.ToLower(text[m[2]:m[3]])
		if seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// indexInSpans reports whether pos falls within any [start,end) span.
func indexInSpans(pos int, spans [][]int) bool {
	for _, s := range spans {
		if pos >= s[0] && pos < s[1] {
			return true
		}
	}
	return false
}
