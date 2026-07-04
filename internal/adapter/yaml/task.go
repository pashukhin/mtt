package yaml

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	goyaml "gopkg.in/yaml.v3"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// Store is the YAML implementation of mtt.TaskStore: one file per task under
// .mtt/tasks/, with flat per-prefix ID minting. It loads config lazily (for the
// type->prefix map) so Get stays independent of config.
type Store struct {
	root string
}

// NewTaskStore returns a task store rooted at the given project directory.
func NewTaskStore(root string) *Store { return &Store{root: root} }

// Create mints a flat per-prefix ID for the logical task, persists it atomically,
// and returns the stored task.
func (s *Store) Create(t mtt.Task) (mtt.Task, error) {
	_, prefixes, err := Load(s.root)
	if err != nil {
		return mtt.Task{}, err
	}
	prefix := prefixes[t.Type]
	if prefix == "" {
		return mtt.Task{}, fmt.Errorf("type %q: no prefix (unknown or prefixless type)", t.Type)
	}
	id, err := mint(s.root, prefix)
	if err != nil {
		return mtt.Task{}, err
	}
	t.ID = id
	data, err := goyaml.Marshal(fromDomainTask(t))
	if err != nil {
		return mtt.Task{}, fmt.Errorf("marshal task %s: %w", id, err)
	}
	path := filepath.Join(s.root, dirName, tasksDirName, id+".yaml")
	if err := atomicWrite(path, data); err != nil {
		return mtt.Task{}, err
	}
	return t, nil
}

// Get loads a task by ID, returning mtt.ErrNotFound when the file is absent.
func (s *Store) Get(id string) (mtt.Task, error) {
	path := filepath.Join(s.root, dirName, tasksDirName, id+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return mtt.Task{}, mtt.ErrNotFound
		}
		return mtt.Task{}, fmt.Errorf("read %s: %w", path, err)
	}
	var yt ymlTask
	if err := goyaml.Unmarshal(data, &yt); err != nil {
		return mtt.Task{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return yt.toDomain()
}
