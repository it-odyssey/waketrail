// Package localdata owns paths and permissions for WakeTrail's local evidence.
package localdata

import (
	"fmt"
	"os"
	"path/filepath"
)

// Directory applies permissions to WakeTrail's own directory and known files,
// including older installations. It never changes XDG_STATE_HOME itself.
func Directory() (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "state")
	}
	dir := filepath.Join(base, "waketrail")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("local evidence directory must be a directory: %s", dir)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return "", err
	}
	for _, name := range []string{"waketrail.db", "waketrail.db-journal", "waketrail.db-wal", "waketrail.db-shm", "active-session.json", "watch.json", "docker-snapshot.json", "watch.log"} {
		path := filepath.Join(dir, name)
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("local evidence must be a regular file: %s", path)
		}
		if err := os.Chmod(path, 0600); err != nil && !os.IsNotExist(err) {
			return "", err
		}
	}
	return dir, nil
}

// OpenPrivate repairs existing permissions before callers write new evidence.
// It rejects symlinks rather than modifying a different file's permissions.
func OpenPrivate(path string, flags int) (*os.File, error) {
	info, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err == nil && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("local evidence must be a regular file: %s", path)
	}
	file, err := os.OpenFile(path, flags&^os.O_TRUNC, 0600)
	if err != nil {
		return nil, err
	}
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return nil, err
	}
	if flags&os.O_TRUNC != 0 {
		if err := file.Truncate(0); err != nil {
			file.Close()
			return nil, err
		}
	}
	return file, nil
}

func WritePrivate(path string, data []byte) error {
	file, err := OpenPrivate(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC)
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
