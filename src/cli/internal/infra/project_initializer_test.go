package infra

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitializeIgnoresEngineRuntimeDirectories(t *testing.T) {
	targetDir := t.TempDir()
	if err := NewFSProjectInitializer().Initialize(targetDir); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(targetDir, ".sdd", "work-items", ".gitignore"))
	if err != nil {
		t.Fatalf("ReadFile() work-items/.gitignore error = %v", err)
	}
	if string(data) != workItemsGitignore {
		t.Fatalf("work-items/.gitignore = %q, want %q", data, workItemsGitignore)
	}
}
