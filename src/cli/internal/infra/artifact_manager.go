package infra

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/ports"
)

type ArtifactManager struct{}

func NewArtifactManager() *ArtifactManager {
	return &ArtifactManager{}
}

func (am *ArtifactManager) PrepareArtifactsForPhase(
	baseDir string,
	workflow *domain.Workflow,
	phaseID string,
	workItemID string,
	templateVars map[string]string,
) ([]ports.ArtifactWrite, error) {
	if err := domain.ValidateIdentifier("work item id", workItemID); err != nil {
		return nil, err
	}
	phase, exists := workflow.Phase(phaseID)
	if !exists {
		return nil, fmt.Errorf("phase %s not found in workflow %s", phaseID, workflow.ID)
	}

	writes := make([]ports.ArtifactWrite, 0, len(phase.Produces))
	for _, artifactID := range phase.Produces {
		artifactConfig, exists := workflow.Artifacts[artifactID]
		if !exists {
			return nil, fmt.Errorf("artifact %s defined in phase but not in workflow artifacts", artifactID)
		}

		templateContent, err := am.readLocalTemplate(baseDir, artifactConfig.Template)
		if err != nil {
			return nil, fmt.Errorf("failed to read template for artifact %s: %w", artifactID, err)
		}

		vars := copyTemplateVars(templateVars)
		vars["artifact_id"] = artifactID
		vars["phase"] = phaseID
		vars["created_by_kind"] = string(domain.ActorCLI)
		vars["created_by_id"] = "sdd"
		vars["sources"] = artifactSources(workflow, phase)

		renderedContent := domain.RenderTemplate(templateContent, vars)
		if strings.Contains(renderedContent, "{{") {
			return nil, fmt.Errorf("%w: template %s contains unresolved placeholders", domain.ErrSchemaValidation, artifactConfig.Template)
		}
		metadata, err := extractFrontMatter(renderedContent)
		if err != nil {
			return nil, fmt.Errorf("%w: artifact %s: %v", domain.ErrSchemaValidation, artifactID, err)
		}
		if err := NewSchemaValidator().ValidateYAML(baseDir, "artifact.schema.json", metadata); err != nil {
			return nil, err
		}
		if err := validateArtifactMetadata(metadata, artifactID, phaseID, workItemID); err != nil {
			return nil, err
		}

		writes = append(writes, ports.ArtifactWrite{
			Path:    artifactConfig.Path,
			Content: []byte(renderedContent),
			Mode:    0644,
		})
	}

	return writes, nil
}

func (am *ArtifactManager) ResolveExternalArtifact(path string) (ports.ExternalArtifact, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return ports.ExternalArtifact{}, fmt.Errorf("%w: resolve path: %v", domain.ErrInvalidExternalArtifact, err)
	}
	info, err := os.Stat(absolutePath)
	if err != nil {
		return ports.ExternalArtifact{}, fmt.Errorf("%w: inspect %s: %v", domain.ErrInvalidExternalArtifact, absolutePath, err)
	}
	if !info.Mode().IsRegular() {
		return ports.ExternalArtifact{}, fmt.Errorf("%w: %s is not a regular file", domain.ErrInvalidExternalArtifact, absolutePath)
	}
	content, err := os.ReadFile(absolutePath)
	if err != nil {
		return ports.ExternalArtifact{}, fmt.Errorf("%w: read %s: %v", domain.ErrInvalidExternalArtifact, absolutePath, err)
	}
	hash := sha256.Sum256(content)
	return ports.ExternalArtifact{
		Path:    absolutePath,
		SHA256:  fmt.Sprintf("%x", hash),
		Content: content,
	}, nil
}

func (am *ArtifactManager) ImportExternalArtifact(
	workflow *domain.Workflow,
	phaseID string,
	artifactID string,
	source ports.ExternalArtifact,
	writes []ports.ArtifactWrite,
) ([]ports.ArtifactWrite, error) {
	phase, exists := workflow.Phase(phaseID)
	if !exists {
		return nil, domain.ErrPhaseNotFound
	}
	producesArtifact := false
	for _, producedID := range phase.Produces {
		if producedID == artifactID {
			producesArtifact = true
			break
		}
	}
	if !producesArtifact {
		return nil, fmt.Errorf("%w: phase %s does not produce artifact %s", domain.ErrInvalidExternalArtifact, phaseID, artifactID)
	}

	artifactConfig := workflow.Artifacts[artifactID]
	writeIndex := -1
	for i := range writes {
		if writes[i].Path == artifactConfig.Path {
			writeIndex = i
			break
		}
	}
	if writeIndex < 0 {
		return nil, fmt.Errorf("generated artifact %s was not prepared", artifactID)
	}
	metadata, err := extractFrontMatter(string(writes[writeIndex].Content))
	if err != nil {
		return nil, fmt.Errorf("%w: generated artifact %s: %v", domain.ErrSchemaValidation, artifactID, err)
	}

	importedContent := "---\n" + string(metadata) + "\n---\n\n" + stripFrontMatter(string(source.Content))
	writes[writeIndex].Content = []byte(importedContent)
	return writes, nil
}

func (am *ArtifactManager) readLocalTemplate(baseDir, templateName string) (string, error) {
	if err := domain.ValidateIdentifier("template id", templateName); err != nil {
		return "", err
	}
	templatePath, err := containedPath(filepath.Join(baseDir, ".sdd"), "templates", templateName+".md")
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template file %s: %w", templatePath, err)
	}

	return string(data), nil
}

func copyTemplateVars(source map[string]string) map[string]string {
	result := make(map[string]string, len(source)+6)
	for key, value := range source {
		result[key] = value
	}
	return result
}

func artifactSources(workflow *domain.Workflow, phase domain.WorkflowPhase) string {
	if len(phase.Requires) == 0 {
		return "[]"
	}
	var lines []string
	for _, requiredPhaseID := range phase.Requires {
		sourcePath := workflow.ArtifactPathForPhase(requiredPhaseID)
		if sourcePath != "" {
			lines = append(lines, "  - "+sourcePath)
		}
	}
	if len(lines) == 0 {
		return "[]"
	}
	return "\n" + strings.Join(lines, "\n")
}

func extractFrontMatter(content string) ([]byte, error) {
	if !strings.HasPrefix(content, "---\n") {
		return nil, fmt.Errorf("missing YAML front matter")
	}
	remaining := content[len("---\n"):]
	end := strings.Index(remaining, "\n---")
	if end < 0 {
		return nil, fmt.Errorf("unterminated YAML front matter")
	}
	return []byte(remaining[:end]), nil
}

func stripFrontMatter(content string) string {
	if !strings.HasPrefix(content, "---\n") {
		return strings.TrimSpace(content) + "\n"
	}
	remaining := content[len("---\n"):]
	end := strings.Index(remaining, "\n---")
	if end < 0 {
		return strings.TrimSpace(content) + "\n"
	}
	return strings.TrimSpace(remaining[end+len("\n---"):]) + "\n"
}

func validateArtifactMetadata(data []byte, artifactID, phaseID, workItemID string) error {
	var metadata struct {
		ID       string `yaml:"id"`
		Phase    string `yaml:"phase"`
		WorkItem string `yaml:"work_item"`
	}
	if err := yaml.Unmarshal(data, &metadata); err != nil {
		return fmt.Errorf("%w: parse artifact metadata: %v", domain.ErrSchemaValidation, err)
	}
	if metadata.ID != artifactID || metadata.Phase != phaseID || metadata.WorkItem != workItemID {
		return fmt.Errorf(
			"%w: artifact metadata mismatch: id=%s phase=%s work_item=%s",
			domain.ErrSchemaValidation,
			metadata.ID,
			metadata.Phase,
			metadata.WorkItem,
		)
	}
	return nil
}
