package yaml

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	goyaml "gopkg.in/yaml.v3"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// Current is the YAML adapter's mtt.CurrentStore: it owns only the top-level
// `current:` key of .mtt/config.local.yaml, reading and writing via a yaml.Node so
// author, comments, and any other local keys survive a rewrite (the file is
// human-edited). It is independent of Load (which ignores the unknown key).
type Current struct {
	root string
}

// NewCurrent returns a current-task store rooted at the given project directory.
func NewCurrent(root string) *Current { return &Current{root: root} }

var _ mtt.CurrentStore = (*Current)(nil)

func (c *Current) path() string { return filepath.Join(c.root, dirName, localConfigName) }

// Current returns the stored current task id; ok is false when the file or the
// `current:` key is absent (not an error).
func (c *Current) Current() (mtt.TaskID, bool, error) {
	data, err := os.ReadFile(c.path())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read %s: %w", c.path(), err)
	}
	var doc goyaml.Node
	if err := goyaml.Unmarshal(data, &doc); err != nil {
		return "", false, fmt.Errorf("parse %s: %w", c.path(), err)
	}
	root := documentRoot(&doc)
	if root == nil {
		return "", false, nil
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "current" {
			v := root.Content[i+1].Value
			if v == "" {
				return "", false, nil
			}
			id, err := mtt.NewTaskID(v)
			return id, err == nil, err
		}
	}
	return "", false, nil
}

// SetCurrent upserts `current: <id>` (creating the file if absent).
func (c *Current) SetCurrent(id mtt.TaskID) error {
	return c.upsert(func(root *goyaml.Node) {
		for i := 0; i+1 < len(root.Content); i += 2 {
			if root.Content[i].Value == "current" {
				root.Content[i+1].Value = string(id)
				root.Content[i+1].Tag = "!!str"
				return
			}
		}
		root.Content = append(root.Content,
			&goyaml.Node{Kind: goyaml.ScalarNode, Value: "current"},
			&goyaml.Node{Kind: goyaml.ScalarNode, Value: string(id), Tag: "!!str"})
	})
}

// ClearCurrent removes the `current:` key, leaving every other key intact.
func (c *Current) ClearCurrent() error {
	return c.upsert(func(root *goyaml.Node) {
		for i := 0; i+1 < len(root.Content); i += 2 {
			if root.Content[i].Value == "current" {
				root.Content = append(root.Content[:i], root.Content[i+2:]...)
				return
			}
		}
	})
}

// upsert reads config.local (or starts an empty mapping), applies mutate to the
// root mapping node, and writes it back atomically — preserving comments/keys.
func (c *Current) upsert(mutate func(root *goyaml.Node)) error {
	var doc goyaml.Node
	data, err := os.ReadFile(c.path())
	switch {
	case err == nil:
		if err := goyaml.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("parse %s: %w", c.path(), err)
		}
	case errors.Is(err, os.ErrNotExist):
		// leave doc zero; ensureMapping builds a fresh document+mapping
	default:
		return fmt.Errorf("read %s: %w", c.path(), err)
	}
	mutate(ensureMapping(&doc))
	out, err := goyaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", c.path(), err)
	}
	return atomicWrite(c.path(), out)
}

// documentRoot returns the root mapping node of a parsed document (nil if empty).
func documentRoot(doc *goyaml.Node) *goyaml.Node {
	if doc.Kind == goyaml.DocumentNode {
		if len(doc.Content) == 0 {
			return nil
		}
		return doc.Content[0]
	}
	if doc.Kind == 0 {
		return nil
	}
	return doc
}

// ensureMapping guarantees doc is a document node whose first child is a mapping,
// and returns that mapping (so an absent/empty file yields a fresh `{}`).
func ensureMapping(doc *goyaml.Node) *goyaml.Node {
	if doc.Kind == 0 {
		doc.Kind = goyaml.DocumentNode
	}
	if len(doc.Content) == 0 {
		doc.Content = []*goyaml.Node{{Kind: goyaml.MappingNode}}
	}
	return doc.Content[0]
}
