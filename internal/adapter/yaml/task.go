package yaml

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	goyaml "gopkg.in/yaml.v3"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// idPattern is the YAML adapter's on-disk id encoding: a type prefix (letters,
// see dto.go's prefixPattern) followed by a positive sequence number — mint's
// <prefix><N>. It doubles as the shell-safety guard: a task id is expanded into
// {{.ID}} inside gate/post shell commands (run via sh -c), so an id bearing
// shell metacharacters or whitespace must never cross into the domain.
var idPattern = regexp.MustCompile(`^[a-zA-Z]+[0-9]+$`)

// parseTaskFile maps one task file's bytes to the domain, enforcing the store's
// id invariants before the mapping: the in-file id: must equal the filename stem
// (no id/filename split-brain — a duplicate id in another file) and match the
// adapter's id encoding (no injection payload riding {{.ID}}). path supplies the
// stem and the error context.
func parseTaskFile(path string, data []byte) (mtt.Task, error) {
	var yt ymlTask
	if err := goyaml.Unmarshal(data, &yt); err != nil {
		return mtt.Task{}, fmt.Errorf("parse %s: %w", path, err)
	}
	stem := strings.TrimSuffix(filepath.Base(path), ".yaml")
	if yt.ID != stem {
		return mtt.Task{}, fmt.Errorf("%s: id %q does not match filename", path, yt.ID)
	}
	if !idPattern.MatchString(yt.ID) {
		return mtt.Task{}, fmt.Errorf("%s: invalid task id %q (want <prefix><number>)", path, yt.ID)
	}
	task, err := yt.toDomain()
	if err != nil {
		return mtt.Task{}, fmt.Errorf("%s: %w", path, err)
	}
	return task, nil
}

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
	_, settings, err := Load(s.root)
	if err != nil {
		return mtt.Task{}, err
	}
	prefix := settings.Prefixes[string(t.Type)]
	if prefix == "" {
		return mtt.Task{}, fmt.Errorf("type %q: no prefix (unknown or prefixless type)", t.Type)
	}
	id, err := mint(s.root, prefix)
	if err != nil {
		return mtt.Task{}, err
	}
	t.ID = mtt.TaskID(id)
	return s.write(t)
}

// write serializes t and atomically persists it to .mtt/tasks/<id>.yaml. t.ID
// must already be set. It is the single serialization+write path for the store.
func (s *Store) write(t mtt.Task) (mtt.Task, error) {
	data, err := goyaml.Marshal(fromDomainTask(t))
	if err != nil {
		return mtt.Task{}, fmt.Errorf("marshal task %s: %w", t.ID, err)
	}
	path := filepath.Join(s.root, dirName, tasksDirName, string(t.ID)+".yaml")
	if err := atomicWrite(path, data); err != nil {
		return mtt.Task{}, err
	}
	return t, nil
}

// Update overwrites an existing task by t.ID; it never mints and never creates.
// A task that does not exist yields mtt.ErrNotFound.
func (s *Store) Update(t mtt.Task) (mtt.Task, error) {
	path := filepath.Join(s.root, dirName, tasksDirName, string(t.ID)+".yaml")
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return mtt.Task{}, mtt.ErrNotFound
		}
		return mtt.Task{}, fmt.Errorf("stat %s: %w", path, err)
	}
	return s.write(t)
}

// Delete removes the task file .mtt/tasks/<id>.yaml. A task that does not exist
// yields mtt.ErrNotFound. The os.Remove unlink is atomic (same filesystem
// assumptions as the store's temp+rename writes).
func (s *Store) Delete(id mtt.TaskID) error {
	path := filepath.Join(s.root, dirName, tasksDirName, string(id)+".yaml")
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return mtt.ErrNotFound
		}
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
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
		if len(data) == 0 {
			continue // mint reserve artifact (c18) — an id reserved but never written; not corruption
		}
		task, err := parseTaskFile(path, data)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// Get loads a task by ID, returning mtt.ErrNotFound when the file is absent —
// or zero-byte: a mint reserve artifact (c18) is "no such task", not corruption.
func (s *Store) Get(id mtt.TaskID) (mtt.Task, error) {
	path := filepath.Join(s.root, dirName, tasksDirName, string(id)+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return mtt.Task{}, mtt.ErrNotFound
		}
		return mtt.Task{}, fmt.Errorf("read %s: %w", path, err)
	}
	if len(data) == 0 {
		return mtt.Task{}, mtt.ErrNotFound
	}
	return parseTaskFile(path, data)
}
