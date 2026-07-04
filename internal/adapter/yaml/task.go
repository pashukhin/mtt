package yaml

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	return s.write(t)
}

// write serializes t and atomically persists it to .mtt/tasks/<id>.yaml. t.ID
// must already be set. It is the single serialization+write path for the store.
func (s *Store) write(t mtt.Task) (mtt.Task, error) {
	data, err := goyaml.Marshal(fromDomainTask(t))
	if err != nil {
		return mtt.Task{}, fmt.Errorf("marshal task %s: %w", t.ID, err)
	}
	path := filepath.Join(s.root, dirName, tasksDirName, t.ID+".yaml")
	if err := atomicWrite(path, data); err != nil {
		return mtt.Task{}, err
	}
	return t, nil
}

// Update overwrites an existing task by t.ID; it never mints and never creates.
// A task that does not exist yields mtt.ErrNotFound.
func (s *Store) Update(t mtt.Task) (mtt.Task, error) {
	path := filepath.Join(s.root, dirName, tasksDirName, t.ID+".yaml")
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return mtt.Task{}, mtt.ErrNotFound
		}
		return mtt.Task{}, fmt.Errorf("stat %s: %w", path, err)
	}
	return s.write(t)
}

// List returns all tasks under .mtt/tasks/, mapping each file to the domain. The
// order is unspecified (os.ReadDir's lexical order); callers impose their own
// deterministic order. A missing tasks/ directory yields an empty slice.
func (s *Store) List() ([]mtt.Task, error) {
	dir := filepath.Join(s.root, dirName, tasksDirName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	var tasks []mtt.Task
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		var yt ymlTask
		if err := goyaml.Unmarshal(data, &yt); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		task, err := yt.toDomain()
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
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
