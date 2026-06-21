package state

import (
	"os"
	"path/filepath"
)

const (
	DirName = ".mydaemon"
	PIDName = "mydaemon.pid"
	LogName = "mydaemon.log"
)

func DefaultStateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, DirName), nil
}

func PIDPath(root string) string {
	return filepath.Join(root, PIDName)
}

func LogPath(root string) string {
	return filepath.Join(root, LogName)
}
