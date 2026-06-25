package state

import (
	"os"
	"path/filepath"
)

const (
	DirName     = ".mydaemon"
	PIDName     = "mydaemon.pid"
	LogName     = "mydaemon.log"
	TasksName   = "tasks.json"
	RuntimeName = "runtime.json"
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

func TasksPath(root string) string {
	return filepath.Join(root, TasksName)
}

func RuntimePath(root string) string {
	return filepath.Join(root, RuntimeName)
}
