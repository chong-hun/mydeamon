package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func WritePID(path string, pid int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o644)
}

func ReadPID(path string) (int, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, true, err
	}
	if pid <= 0 {
		return 0, true, fmt.Errorf("pid must be positive: %d", pid)
	}

	return pid, true, nil
}

func RemovePID(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}

	return err
}
