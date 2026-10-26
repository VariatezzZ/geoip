package atomicfile

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func Write(path string, data []byte, mode fs.FileMode) error {
	if path == "" {
		return fmt.Errorf("output path is empty")
	}
	if mode == 0 {
		mode = 0o644
	}

	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create output directory %s: %w", directory, err)
	}

	temporary, err := os.CreateTemp(directory, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary file for %s: %w", path, err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	defer temporary.Close()

	if err := temporary.Chmod(mode); err != nil {
		return fmt.Errorf("set permissions on temporary file for %s: %w", path, err)
	}
	if _, err := temporary.Write(data); err != nil {
		return fmt.Errorf("write temporary file for %s: %w", path, err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync temporary file for %s: %w", path, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary file for %s: %w", path, err)
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return fmt.Errorf("replace output %s: %w", path, err)
	}
	return nil
}
