package scaffold

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
)

//go:embed docdata/agents.md
var runbookContent string

//go:embed docdata/pointer.md
var pointerContent string

// Block markers (HTML comments — invisible in rendered markdown).
const (
	blockBegin = "<!-- mtt:begin -->"
	blockEnd   = "<!-- mtt:end -->"
)

// docTarget scaffolds one markdown doc file by injecting a marked block.
type docTarget struct {
	name    string
	relPath string
	content string
}

func (t docTarget) Name() string    { return t.name }
func (t docTarget) RelPath() string { return t.relPath }
func (t docTarget) Merge(existing []byte, exists bool) ([]byte, Action, error) {
	return blockMerge(existing, exists, t.content)
}

// DocTargets returns the agent-doc scaffold targets: the runbook (AGENTS.md) and
// the pointer stubs (CLAUDE.md, GEMINI.md, sharing the pointer content).
func DocTargets() []Target {
	return []Target{
		docTarget{name: "agents", relPath: "AGENTS.md", content: runbookContent},
		docTarget{name: "claude", relPath: "CLAUDE.md", content: pointerContent},
		docTarget{name: "gemini", relPath: "GEMINI.md", content: pointerContent},
	}
}

// canonicalBlock wraps content in the markers, normalizing content to end with
// exactly one newline: begin\n + content + \n + end\n.
func canonicalBlock(content string) string {
	body := strings.TrimRight(content, "\n") + "\n"
	return blockBegin + "\n" + body + blockEnd + "\n"
}

// blockMerge injects/regenerates the marked block into existing markdown. Pure:
// create-if-absent(or empty), append-if-no-markers, regenerate-if-one-pair,
// refuse a malformed marker pair — never clobbering surrounding content.
func blockMerge(existing []byte, exists bool, content string) ([]byte, Action, error) {
	block := canonicalBlock(content)
	if !exists || len(bytes.TrimSpace(existing)) == 0 {
		return []byte(block), Created, nil
	}
	s := string(existing)
	nBegin := strings.Count(s, blockBegin)
	nEnd := strings.Count(s, blockEnd)
	switch {
	case nBegin == 0 && nEnd == 0:
		prefix := s
		if !strings.HasSuffix(prefix, "\n") {
			prefix += "\n"
		}
		return []byte(prefix + "\n" + block), Merged, nil
	case nBegin == 1 && nEnd == 1:
		i := strings.Index(s, blockBegin)
		j := strings.Index(s, blockEnd)
		if j < i {
			return nil, "", fmt.Errorf("cannot merge: %q appears before %q", blockEnd, blockBegin)
		}
		k := j + len(blockEnd)
		if k < len(s) && s[k] == '\n' {
			k++
		}
		out := s[:i] + block + s[k:]
		if out == s {
			return existing, Unchanged, nil
		}
		return []byte(out), Merged, nil
	default:
		return nil, "", fmt.Errorf("cannot merge: %d %q / %d %q markers (expected one pair)",
			nBegin, blockBegin, nEnd, blockEnd)
	}
}
