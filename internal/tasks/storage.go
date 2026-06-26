package tasks

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
)

type Store struct {
	path string
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Load() (Collection, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return Collection{}, nil
	}
	if err != nil {
		return Collection{}, err
	}

	var collection Collection
	if err := json.Unmarshal(data, &collection); err != nil {
		return Collection{}, err
	}

	return collection, nil
}

func (s *Store) Save(collection Collection) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(collection, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(s.path), "tasks-*.json")
	if err != nil {
		return err
	}

	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	n, err := tmp.Write(data)
	if err != nil {
		_ = tmp.Close()
		return err
	}
	if n != len(data) {
		_ = tmp.Close()
		return io.ErrShortWrite
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpPath, s.path)
}
