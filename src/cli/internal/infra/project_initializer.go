package infra

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"sdd-cli/embeds"
)

type FSProjectInitializer struct{}

func NewFSProjectInitializer() *FSProjectInitializer {
	return &FSProjectInitializer{}
}

func (initializer *FSProjectInitializer) Initialize(targetDir string) (resultErr error) {
	sddDir := filepath.Join(targetDir, ".sdd")
	if _, err := os.Stat(sddDir); err == nil {
		return fmt.Errorf(".sdd directory already exists in target path")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to inspect target .sdd directory: %w", err)
	}

	subFS, err := fs.Sub(embeds.DefaultSDDResources, "default_sdd")
	if err != nil {
		return fmt.Errorf("failed to access embedded default_sdd: %w", err)
	}
	stageDir, err := os.MkdirTemp(targetDir, ".sdd-init-")
	if err != nil {
		return fmt.Errorf("failed to create .sdd staging directory: %w", err)
	}
	stageOwned := true
	defer func() {
		if stageOwned {
			resultErr = errors.Join(resultErr, os.RemoveAll(stageDir))
		}
	}()

	err = fs.WalkDir(subFS, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == "work-items" || filepath.HasPrefix(path, "work-items/") ||
			path == "tests" || filepath.HasPrefix(path, "tests/") {
			return nil
		}

		targetPath := filepath.Join(stageDir, path)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		data, err := fs.ReadFile(subFS, path)
		if err != nil {
			return fmt.Errorf("failed to read embedded file %s: %w", path, err)
		}
		return writeInitializedFile(targetPath, data)
	})
	if err != nil {
		return fmt.Errorf("failed to unpack .sdd template: %w", err)
	}

	for _, path := range []string{
		filepath.Join(stageDir, "work-items", "active"),
		filepath.Join(stageDir, "work-items", "archive"),
	} {
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", path, err)
		}
		// Git does not track empty directories; keep them present after clone/worktree checkout.
		if err := writeInitializedFile(filepath.Join(path, ".gitkeep"), nil); err != nil {
			return fmt.Errorf("failed to create %s/.gitkeep: %w", path, err)
		}
	}

	if err := os.Rename(stageDir, sddDir); err != nil {
		return fmt.Errorf("failed to publish .sdd directory: %w", err)
	}
	stageOwned = false
	return nil
}

func writeInitializedFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := file.Sync(); err != nil {
		return errors.Join(err, file.Close())
	}
	return file.Close()
}
