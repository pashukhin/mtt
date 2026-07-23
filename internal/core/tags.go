package core

import (
	"sort"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// canonicalTags merges tag groups into one normalized, deduped, SORTED set — the
// canonical on-disk form. Each value is normalized via mtt.NormalizeTag; invalid
// ones are dropped (explicit tags are validated at the CLI boundary, so this is
// defensive). Returns nil when the result is empty.
func canonicalTags(groups ...[]string) []string {
	seen := map[string]bool{}
	for _, g := range groups {
		for _, raw := range g {
			if tag, ok := mtt.NormalizeTag(raw); ok {
				seen[tag] = true
			}
		}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for tag := range seen {
		out = append(out, tag)
	}
	sort.Strings(out)
	return out
}

// tagSet builds a membership set from tag groups (values assumed already normalized).
func tagSet(groups ...[]string) map[string]bool {
	m := map[string]bool{}
	for _, g := range groups {
		for _, tag := range g {
			m[tag] = true
		}
	}
	return m
}

// subtractTags returns the elements of a (a canonical set) not present in b,
// preserving a's order — used to report the tags a mutation actually changed.
func subtractTags(a, b []string) []string {
	if len(a) == 0 {
		return nil
	}
	bset := tagSet(b)
	var out []string
	for _, tag := range a {
		if !bset[tag] {
			out = append(out, tag)
		}
	}
	return out
}

// reconcileTags recomputes a task's tags after a text edit: it drops tags whose
// #hashtag left the text (present in the old text, absent from the new) and adds
// tags whose #hashtag entered it, preserving manual tags (never in the old text).
// Returns the canonical set.
func reconcileTags(current []string, oldTitle, oldDesc, newTitle, newDesc string) []string {
	oldText := tagSet(mtt.ExtractTags(oldTitle), mtt.ExtractTags(oldDesc))
	newTags := append(mtt.ExtractTags(newTitle), mtt.ExtractTags(newDesc)...)
	newSet := tagSet(newTags)
	kept := make([]string, 0, len(current)+len(newTags))
	for _, tag := range current {
		if oldText[tag] && !newSet[tag] {
			continue // its only anchor left the text
		}
		kept = append(kept, tag)
	}
	return canonicalTags(kept, newTags)
}

// TagCount is one tag and how many of the counted tasks carry it.
type TagCount struct {
	Tag   string
	Count int
}

// TagCounts tallies tags across tasks and returns them sorted by Count
// descending, then Tag ascending. A task's Tags are a normalized set (no
// duplicates), so each task contributes at most once per tag.
func TagCounts(tasks []mtt.Task) []TagCount {
	counts := map[string]int{}
	for _, t := range tasks {
		for _, tag := range t.Tags {
			counts[tag]++
		}
	}
	out := make([]TagCount, 0, len(counts))
	for tag, n := range counts {
		out = append(out, TagCount{Tag: tag, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Tag < out[j].Tag
	})
	return out
}
