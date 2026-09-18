package infra

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sdd-cli/internal/domain"
)

func TestFSAdapterRepositoryListsClaudeCode(t *testing.T) {
	adapters, err := NewFSAdapterRepository().ListAdapters()
	if err != nil {
		t.Fatalf("ListAdapters() error = %v", err)
	}
	if len(adapters) != 1 {
		t.Fatalf("adapters = %#v, want one adapter", adapters)
	}
	if adapters[0].ID != "claude-code" || adapters[0].Title != "Claude Code" {
		t.Fatalf("adapter = %#v", adapters[0])
	}
}

func TestFSAdapterRepositoryInstallsClaudeCode(t *testing.T) {
	targetDir := t.TempDir()
	if err := NewFSProjectInitializer().Initialize(targetDir); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	result, err := NewFSAdapterRepository().InstallAdapter(targetDir, "claude-code")
	if err != nil {
		t.Fatalf("InstallAdapter() error = %v", err)
	}
	if result.ID != "claude-code" || len(result.Files) == 0 {
		t.Fatalf("result = %#v", result)
	}

	expectedFiles := []string{
		"CLAUDE.md",
		".mcp.json",
		".claude/.gitignore",
		".claude/settings.json",
		".claude/settings.local.json.example",
		".claude/agents/sdd-orchestrator.md",
		".claude/skills/create-plan/SKILL.md",
		".claude/commands/sdd/feature.md",
		".claude/hooks/.gitkeep",
		".claude/rules/.gitkeep",
	}
	for _, relative := range expectedFiles {
		if _, err := os.Stat(filepath.Join(targetDir, filepath.FromSlash(relative))); err != nil {
			t.Fatalf("installed file %s missing: %v", relative, err)
		}
		if !containsString(result.Files, relative) {
			t.Fatalf("result files missing %s: %#v", relative, result.Files)
		}
	}
	ignore, err := os.ReadFile(filepath.Join(targetDir, ".claude", ".gitignore"))
	if err != nil || strings.TrimSpace(string(ignore)) != "settings.local.json" {
		t.Fatalf(".claude/.gitignore = %q, err = %v", ignore, err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, ".claude", "settings.local.json")); !os.IsNotExist(err) {
		t.Fatalf("private settings file should not be installed: %v", err)
	}
}

func TestFSAdapterRepositoryRequiresInitializedProject(t *testing.T) {
	_, err := NewFSAdapterRepository().InstallAdapter(t.TempDir(), "claude-code")
	if !errors.Is(err, domain.ErrInvalidPath) {
		t.Fatalf("InstallAdapter() error = %v, want %v", err, domain.ErrInvalidPath)
	}
}

func TestFSAdapterRepositoryRejectsUnknownAdapter(t *testing.T) {
	_, err := NewFSAdapterRepository().InstallAdapter(t.TempDir(), "unknown")
	if !errors.Is(err, domain.ErrAdapterNotFound) {
		t.Fatalf("InstallAdapter() error = %v, want %v", err, domain.ErrAdapterNotFound)
	}
}

func TestFSAdapterRepositoryRejectsCollisionWithoutPartialInstall(t *testing.T) {
	targetDir := t.TempDir()
	if err := NewFSProjectInitializer().Initialize(targetDir); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	existing := []byte("project instructions\n")
	if err := os.WriteFile(filepath.Join(targetDir, "CLAUDE.md"), existing, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := NewFSAdapterRepository().InstallAdapter(targetDir, "claude-code")
	if !errors.Is(err, domain.ErrAdapterInstallConflict) {
		t.Fatalf("InstallAdapter() error = %v, want %v", err, domain.ErrAdapterInstallConflict)
	}
	content, readErr := os.ReadFile(filepath.Join(targetDir, "CLAUDE.md"))
	if readErr != nil || string(content) != string(existing) {
		t.Fatalf("existing CLAUDE.md changed: content=%q err=%v", content, readErr)
	}
	for _, relative := range []string{".mcp.json", ".claude/settings.json"} {
		if _, statErr := os.Stat(filepath.Join(targetDir, filepath.FromSlash(relative))); !os.IsNotExist(statErr) {
			t.Fatalf("partial adapter file %s exists: %v", relative, statErr)
		}
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
