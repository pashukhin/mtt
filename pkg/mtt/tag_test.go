package mtt

import (
	"reflect"
	"testing"
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
	}
	for _, c := range cases {
		got := ExtractTags(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("ExtractTags(%q) = %v; want %v", c.in, got, c.want)
		}
	}
}
