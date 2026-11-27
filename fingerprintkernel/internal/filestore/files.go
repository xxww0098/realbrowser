package filestore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// WriteAtomic replaces a file after the temporary file has been closed.
func WriteAtomic(dir, name string, data []byte) error {
	tmp, err := os.CreateTemp(dir, ".write-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(data)
	if err = errors.Join(err, tmp.Close()); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, name))
}

// ValidDir reports whether the caller supplied an explicit directory.
func ValidDir(dir string) bool {
	return strings.TrimSpace(dir) != "" && filepath.Clean(dir) != "."
}
