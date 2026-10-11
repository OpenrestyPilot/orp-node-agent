package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"orp-node-agent/internal/controlplane"
)

type resultStore struct{ dir string }

func newResultStore(dir string) (*resultStore, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return nil, err
	}
	return &resultStore{dir: dir}, nil
}

func (s *resultStore) path(taskID string) string {
	sum := sha256.Sum256([]byte(taskID))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:])+".json")
}

func (s *resultStore) Load(taskID string) (*controlplane.TaskResult, error) {
	data, err := os.ReadFile(s.path(taskID))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result controlplane.TaskResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if result.TaskID != taskID {
		return nil, errors.New("cached task result ID mismatch")
	}
	return &result, nil
}

func (s *resultStore) Save(result controlplane.TaskResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, ".result-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, s.path(result.TaskID)); err != nil {
		return err
	}
	directory, err := os.Open(s.dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
