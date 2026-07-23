package mtt

import (
	"reflect"
	"testing"
)

// NFD (decomposed) fixtures built from explicit rune values so the combining mark
// is unambiguous — an editor may silently compose a literal "á"/"й" to NFC, which
// would not exercise the truncation path. nfdAcute = "a"+U+0301 ("á"); nfdBreve =
// "и"+U+0306 ("й"). The mark (\pM) must stay in the token, not truncate it.
var (
	nfdAcute = "a" + string(rune(0x0301)) + "uth" // "áuth"
	nfdBreve = "и" + string(rune(0x0306)) + "ога" // "йога"
)

func TestNormalizeTag(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"auth", "auth", true},
		{"#Auth", "auth", true},
		{"  #Auth  ", "auth", true},
		{"#Бэкенд", "бэкенд", true}, // Unicode case-fold
		{"a.b_c-d", "a.b_c-d", true},
		{"a", "a", true},
		{"1", "1", true},
		{"я", "я", true},
		{"#" + nfdAcute, nfdAcute, true}, // NFD accent kept whole, not truncated to "a"
		{"a b", "", false},
		{"", "", false},
		{"-x", "", false},
		{"x-", "", false},
		{"a!b", "", false},
		{"#", "", false},
	}
	for _, c := range cases {
		got, ok := NormalizeTag(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("NormalizeTag(%q) = (%q, %v); want (%q, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestExtractTags(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"fix #auth in #API", []string{"auth", "api"}},
		{"чиним #бэкенд", []string{"бэкенд"}},
		{"#a #A", []string{"a"}},                    // dedup, case-insensitive
		{"see #auth.", []string{"auth"}},            // trailing punctuation stripped
		{"#a-b_c.d ok", []string{"a-b_c.d"}},        // interior . _ - kept
		{"#include <stdio.h>", []string{"include"}}, // accepted false positive
		{"#L42", []string{"l42"}},                   // accepted false positive
		{"тег#backend", nil},                        // non-ASCII adjacency rejected
		{"café#x", nil},
		{"foo#bar", nil},
		{"a#b", nil},
		{"##heading", nil},
		{"# heading", nil},
		{"#!/bin/sh", nil},
		{"C# rocks", nil},
		{"see host#frag", nil},
		{"no hashtags here", nil},
		{"", nil},
		// A #fragment inside a URL (scheme://…) is not a tag — a pasted link must
		// not mint anchors. The whole scheme://non-space run is skipped.
		{"https://example.com/#Naming", nil},
		{"docs https://ex.com/p/#skip and #keep", []string{"keep"}},
		{"#keep https://ex.com/p/#skip", []string{"keep"}},
		// Boundary: only scheme:// runs are treated as URLs. A schemeless "url-ish"
		// string is still scanned for hashtags (documented, deliberate).
		{"example.com/#x", []string{"x"}},
		// A decomposed (NFD) accent/mark must NOT truncate the token (the combining
		// mark stays part of it). No NFC folding — the NFD and NFC spellings are
		// distinct byte sequences and won't cross-match, but neither truncates.
		{"fix #" + nfdAcute, []string{nfdAcute}},
		{"чиним #" + nfdBreve, []string{nfdBreve}},
	}
	for _, c := range cases {
		got := ExtractTags(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("ExtractTags(%q) = %v; want %v", c.in, got, c.want)
		}
	}
}
