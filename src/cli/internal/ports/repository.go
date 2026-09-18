package ports

import (
	"io/fs"
	"time"

	"sdd-cli/internal/domain"
)

type ArtifactWrite struct {
	Path    string
	Content []byte
	Mode    fs.FileMode
}

type WorkItemCommit struct {
	Item        *domain.WorkItem
	Artifacts   []ArtifactWrite
	Events      []domain.Event
	OperationID string
}

type WorkItemLocation string

const (
	WorkItemLocationActive  WorkItemLocation = "active"
	WorkItemLocationArchive WorkItemLocation = "archive"
)

type LocatedWorkItem struct {
	Item         *domain.WorkItem
	Location     WorkItemLocation
	RelativePath string
}

type WorkItemArchiveCommit struct {
	Item        *domain.WorkItem
	Event       domain.Event
	ArchivedAt  time.Time
	Destination string
	OperationID string
}

type WorkItemReader interface {
	GetWorkItem(baseDir string, id string) (*domain.WorkItem, error)
}

type WorkItemCatalogReader interface {
	FindWorkItem(baseDir string, id string) (*LocatedWorkItem, error)
}

type WorkItemExistenceChecker interface {
	WorkItemExists(baseDir string, id string) (bool, error)
}

type OperationTracker interface {
	OperationApplied(baseDir string, id string, operationID string) (bool, error)
}

type WorkItemCommitter interface {
	CommitWorkItem(baseDir string, commit WorkItemCommit) error
}

type WorkItemMutationRepository interface {
	WorkItemReader
	OperationTracker
	WorkItemCommitter
}

type WorkItemCreationRepository interface {
	WorkItemMutationRepository
	WorkItemExistenceChecker
}

type WorkItemArchiver interface {
	FindArchivedWorkItem(baseDir string, id string) (*LocatedWorkItem, error)
	ArchivedOperationApplied(baseDir string, id string, operationID string) (bool, error)
	ArchiveWorkItem(baseDir string, commit WorkItemArchiveCommit) (*LocatedWorkItem, error)
}

type WorkflowRepository interface {
	GetWorkflow(baseDir string, workflowID string) (*domain.Workflow, error)
}

type ConfigRepository interface {
	GetConfig(baseDir string) (*domain.Config, error)
}

type ValidationInspector interface {
	InspectProject(baseDir string) ([]domain.ValidationCheck, error)
	InspectWorkItem(baseDir string, id string) ([]domain.ValidationCheck, error)
}

type ExternalArtifact struct {
	Path    string
	SHA256  string
	Content []byte
}

type ArtifactPreparer interface {
	PrepareArtifactsForPhase(
		baseDir string,
		workflow *domain.Workflow,
		phaseID string,
		workItemID string,
		templateVars map[string]string,
	) ([]ArtifactWrite, error)
}

type ExternalArtifactImporter interface {
	ResolveExternalArtifact(path string) (ExternalArtifact, error)
	ImportExternalArtifact(
		workflow *domain.Workflow,
		phaseID string,
		artifactID string,
		source ExternalArtifact,
		writes []ArtifactWrite,
	) ([]ArtifactWrite, error)
}

type ArtifactService interface {
	ArtifactPreparer
	ExternalArtifactImporter
}

type ProjectInitializer interface {
	Initialize(targetDir string) error
}

type AdapterDescriptor struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type AdapterInstallation struct {
	ID    string   `json:"id"`
	Files []string `json:"files"`
}

type AdapterCatalog interface {
	ListAdapters() ([]AdapterDescriptor, error)
}

type AdapterInstaller interface {
	InstallAdapter(targetDir string, adapterID string) (*AdapterInstallation, error)
}

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() (string, error)
}
