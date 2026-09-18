package domain

type ApprovalPolicy string

const (
	ApprovalNone     ApprovalPolicy = "none"
	ApprovalRequired ApprovalPolicy = "required"
	ApprovalOptional ApprovalPolicy = "optional"
)

type EntryPoint struct {
	Phase   string   `json:"phase" yaml:"phase"`
	Accepts []string `json:"accepts" yaml:"accepts"`
}

type WorkflowPhase struct {
	ID        string         `json:"id" yaml:"id"`
	Requires  []string       `json:"requires,omitempty" yaml:"requires,omitempty"`
	Produces  []string       `json:"produces,omitempty" yaml:"produces,omitempty"`
	Procedure string         `json:"procedure,omitempty" yaml:"procedure,omitempty"`
	Approval  ApprovalPolicy `json:"approval" yaml:"approval"`
	Optional  bool           `json:"optional,omitempty" yaml:"optional,omitempty"`
	Effects   []string       `json:"effects,omitempty" yaml:"effects,omitempty"`
}

type WorkflowArtifact struct {
	Path     string `json:"path" yaml:"path"`
	Template string `json:"template" yaml:"template"`
}

type Workflow struct {
	SchemaVersion string                      `json:"schema_version" yaml:"schema_version"`
	Kind          string                      `json:"kind" yaml:"kind"`
	ID            string                      `json:"id" yaml:"id"`
	Title         string                      `json:"title" yaml:"title"`
	WorkItemType  string                      `json:"work_item_type" yaml:"work_item_type"`
	Description   string                      `json:"description,omitempty" yaml:"description,omitempty"`
	EntryPoints   []EntryPoint                `json:"entry_points,omitempty" yaml:"entry_points,omitempty"`
	Phases        []WorkflowPhase             `json:"phases" yaml:"phases"`
	Artifacts     map[string]WorkflowArtifact `json:"artifacts" yaml:"artifacts"`
}

func (w *Workflow) Phase(id string) (WorkflowPhase, bool) {
	for _, phase := range w.Phases {
		if phase.ID == id {
			return phase, true
		}
	}

	return WorkflowPhase{}, false
}

func (w *Workflow) ArtifactPathForPhase(phaseID string) string {
	phase, ok := w.Phase(phaseID)
	if !ok {
		return ""
	}

	for _, artifactID := range phase.Produces {
		if artifact, exists := w.Artifacts[artifactID]; exists {
			return artifact.Path
		}
	}

	return ""
}
