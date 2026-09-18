package usecases_test

import (
	"errors"
	"testing"

	"sdd-cli/internal/ports"
	"sdd-cli/internal/usecases"
)

func TestListAdaptersUseCaseDelegatesToCatalog(t *testing.T) {
	repository := &memoryAdapterRepository{
		adapters: []ports.AdapterDescriptor{{
			ID:          "claude-code",
			Title:       "Claude Code",
			Description: "Claude adapter",
		}},
	}

	result, err := usecases.NewListAdaptersUseCase(repository).Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if repository.listCalls != 1 || len(result) != 1 || result[0].ID != "claude-code" {
		t.Fatalf("result = %#v, list calls = %d", result, repository.listCalls)
	}
}

func TestInstallAdapterUseCaseValidatesAndDelegates(t *testing.T) {
	repository := &memoryAdapterRepository{
		installation: &ports.AdapterInstallation{
			ID:    "claude-code",
			Files: []string{"CLAUDE.md"},
		},
	}
	useCase := usecases.NewInstallAdapterUseCase(repository)

	result, err := useCase.Execute("project", "claude-code")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if repository.installCalls != 1 ||
		repository.targetDir != "project" ||
		repository.adapterID != "claude-code" ||
		result.ID != "claude-code" {
		t.Fatalf("repository = %#v, result = %#v", repository, result)
	}

	if _, err := useCase.Execute("project", "../claude"); err == nil {
		t.Fatal("Execute() invalid adapter error = nil")
	}
	if repository.installCalls != 1 {
		t.Fatalf("invalid adapter reached repository, calls = %d", repository.installCalls)
	}
}

func TestAdapterUseCasesPropagateRepositoryFailures(t *testing.T) {
	adapterErr := errors.New("adapter failure")
	repository := &memoryAdapterRepository{err: adapterErr}

	if _, err := usecases.NewListAdaptersUseCase(repository).Execute(); !errors.Is(err, adapterErr) {
		t.Fatalf("List Execute() error = %v, want %v", err, adapterErr)
	}
	if _, err := usecases.NewInstallAdapterUseCase(repository).Execute("project", "claude-code"); !errors.Is(err, adapterErr) {
		t.Fatalf("Install Execute() error = %v, want %v", err, adapterErr)
	}
}

type memoryAdapterRepository struct {
	adapters     []ports.AdapterDescriptor
	installation *ports.AdapterInstallation
	err          error
	listCalls    int
	installCalls int
	targetDir    string
	adapterID    string
}

func (repository *memoryAdapterRepository) ListAdapters() ([]ports.AdapterDescriptor, error) {
	repository.listCalls++
	if repository.err != nil {
		return nil, repository.err
	}
	return repository.adapters, nil
}

func (repository *memoryAdapterRepository) InstallAdapter(targetDir, adapterID string) (*ports.AdapterInstallation, error) {
	repository.installCalls++
	repository.targetDir = targetDir
	repository.adapterID = adapterID
	if repository.err != nil {
		return nil, repository.err
	}
	return repository.installation, nil
}
