package infra

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/ports"
)

const (
	categoryProject   = "project"
	categoryConfig    = "config"
	categoryRegistry  = "registry"
	categorySchema    = "schema"
	categoryWorkflow  = "workflow"
	categoryTemplate  = "template"
	categoryProcedure = "procedure"
	categoryManifest  = "manifest"
	categoryWorkItem  = "work_item"
	categoryArtifact  = "artifact"
	categoryEvent     = "event"
	categoryReference = "reference"
)

var requiredSchemas = []string{
	"artifact.schema.json",
	"event.schema.json",
	"work-item.schema.json",
	"workflow.schema.json",
}

type FSValidationInspector struct {
	schemas *SchemaValidator
}

type validationContext struct {
	baseDir         string
	checks          []domain.ValidationCheck
	schemaAvailable map[string]bool
	workflows       map[string]*domain.Workflow
}

type validationConfig struct {
	SchemaVersion string `yaml:"schema_version"`
	Defaults      struct {
		Workflow string `yaml:"workflow"`
	} `yaml:"defaults"`
}

type capabilityRegistry struct {
	SchemaVersion string                 `yaml:"schema_version"`
	Capabilities  []registeredCapability `yaml:"capabilities"`
}

type registeredCapability struct {
	ID        string `yaml:"id"`
	Title     string `yaml:"title"`
	Procedure string `yaml:"procedure"`
}

func NewFSValidationInspector() ports.ValidationInspector {
	return &FSValidationInspector{schemas: NewSchemaValidator()}
}

func (inspector *FSValidationInspector) InspectProject(baseDir string) ([]domain.ValidationCheck, error) {
	context := newValidationContext(baseDir)
	if !context.inspectDirectory(".sdd", categoryProject, "project.sdd_root_exists", true) {
		return context.checks, nil
	}

	requiredDirectories := []string{
		".sdd/schemas",
		".sdd/workflows",
		".sdd/templates",
		".sdd/procedures",
		".sdd/registry",
		".sdd/work-items/active",
		".sdd/work-items/archive",
	}
	for _, path := range requiredDirectories {
		context.inspectDirectory(path, categoryProject, "project.required_path_exists", true)
	}
	context.inspectRegularFile(".sdd/config.yaml", categoryProject, "project.required_path_exists", true)
	context.inspectRegularFile(".sdd/registry/capabilities.yaml", categoryProject, "project.required_path_exists", true)

	inspector.inspectSchemas(context)
	defaultWorkflow := inspector.inspectConfig(context)
	inspector.inspectCapabilities(context)
	inspector.inspectWorkflows(context, "")
	if defaultWorkflow != "" {
		if _, exists := context.workflows[defaultWorkflow]; exists {
			context.pass(categoryConfig, "config.default_workflow_exists", ".sdd/config.yaml", fmt.Sprintf("default workflow %q is available", defaultWorkflow))
		} else {
			context.fail(categoryConfig, "config.default_workflow_exists", ".sdd/config.yaml", fmt.Sprintf("default workflow %q is not available or invalid", defaultWorkflow))
		}
	}

	activeIDs := make(map[string]struct{})
	activePath := filepath.Join(baseDir, ".sdd", "work-items", "active")
	entries, err := os.ReadDir(activePath)
	if err != nil {
		context.fail(categoryProject, "project.work_items_readable", ".sdd/work-items/active", err.Error())
		return context.checks, nil
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		target := filepath.ToSlash(filepath.Join(".sdd", "work-items", "active", entry.Name()))
		if !entry.IsDir() {
			context.fail(categoryProject, "project.work_item_entry_valid", target, "active work item entry must be a directory")
			continue
		}
		if err := domain.ValidateIdentifier("work item id", entry.Name()); err != nil {
			context.fail(categoryProject, "project.work_item_entry_valid", target, err.Error())
			continue
		}
		context.pass(categoryProject, "project.work_item_entry_valid", target, "work item directory is valid")
		activeIDs[entry.Name()] = struct{}{}
		inspector.inspectWorkItem(
			context,
			entry.Name(),
			false,
			target,
			ports.WorkItemLocationActive,
		)
	}

	archiveIDs := make(map[string]int)
	archivePath := filepath.Join(baseDir, ".sdd", "work-items", "archive")
	archiveEntries, err := os.ReadDir(archivePath)
	if err != nil {
		context.fail(categoryProject, "project.work_items_readable", ".sdd/work-items/archive", err.Error())
		return context.checks, nil
	}
	for _, entry := range archiveEntries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		target := filepath.ToSlash(filepath.Join(".sdd", "work-items", "archive", entry.Name()))
		if !entry.IsDir() {
			context.fail(categoryProject, "archive.entry_valid", target, "archive entry must be a directory")
			continue
		}
		id, err := parseArchiveDirectoryName(entry.Name())
		if err != nil {
			context.fail(categoryProject, "archive.entry_valid", target, err.Error())
			continue
		}
		context.pass(categoryProject, "archive.entry_valid", target, "archive directory name is valid")
		archiveIDs[id]++
		if archiveIDs[id] > 1 {
			context.fail(categoryProject, "archive.id_unique", target, fmt.Sprintf("multiple archive entries found for %s", id))
		}
		if _, exists := activeIDs[id]; exists {
			context.fail(categoryProject, "work_item.location_unique", target, fmt.Sprintf("work item %s exists in active and archive", id))
		}
		inspector.inspectWorkItem(
			context,
			id,
			false,
			target,
			ports.WorkItemLocationArchive,
		)
	}

	return context.checks, nil
}

func (inspector *FSValidationInspector) inspectCapabilities(context *validationContext) {
	target := ".sdd/registry/capabilities.yaml"
	data, err := os.ReadFile(context.absolute(target))
	if err != nil {
		context.fail(categoryRegistry, "registry.yaml_valid", target, err.Error())
		return
	}

	var registry capabilityRegistry
	if err := yaml.Unmarshal(data, &registry); err != nil {
		context.fail(categoryRegistry, "registry.yaml_valid", target, err.Error())
		return
	}
	context.pass(categoryRegistry, "registry.yaml_valid", target, "capability registry is valid YAML")
	if registry.SchemaVersion != "0.1" {
		context.fail(categoryRegistry, "registry.schema_version_supported", target, fmt.Sprintf("unsupported schema version %q", registry.SchemaVersion))
	} else {
		context.pass(categoryRegistry, "registry.schema_version_supported", target, "capability registry schema version is supported")
	}

	seen := make(map[string]struct{}, len(registry.Capabilities))
	for index, capability := range registry.Capabilities {
		capabilityTarget := fmt.Sprintf("%s#capabilities[%d]", target, index)
		if strings.TrimSpace(capability.ID) == "" {
			context.fail(categoryRegistry, "registry.capability_id_valid", capabilityTarget, "capability id cannot be empty")
		} else if _, exists := seen[capability.ID]; exists {
			context.fail(categoryRegistry, "registry.capability_id_unique", capabilityTarget, fmt.Sprintf("duplicate capability id %q", capability.ID))
		} else {
			seen[capability.ID] = struct{}{}
			context.pass(categoryRegistry, "registry.capability_id_unique", capabilityTarget, fmt.Sprintf("capability id %q is unique", capability.ID))
		}
		if strings.TrimSpace(capability.Title) == "" {
			context.fail(categoryRegistry, "registry.capability_title_present", capabilityTarget, "capability title cannot be empty")
		}
		if capability.Procedure == "" {
			continue
		}
		procedurePath := filepath.ToSlash(filepath.Clean(capability.Procedure))
		if procedurePath != capability.Procedure ||
			!strings.HasPrefix(procedurePath, "procedures/") ||
			filepath.Ext(procedurePath) != ".md" {
			context.fail(categoryRegistry, "registry.procedure_reference_valid", capabilityTarget, fmt.Sprintf("invalid procedure reference %q", capability.Procedure))
			continue
		}
		procedureTarget := filepath.ToSlash(filepath.Join(".sdd", filepath.FromSlash(procedurePath)))
		if context.inspectRegularFile(procedureTarget, categoryProcedure, "registry.procedure_file_exists", true) {
			context.pass(categoryRegistry, "registry.procedure_reference_valid", capabilityTarget, fmt.Sprintf("procedure %q is available", capability.Procedure))
		}
	}
}

func (inspector *FSValidationInspector) InspectWorkItem(baseDir, id string) ([]domain.ValidationCheck, error) {
	if err := domain.ValidateIdentifier("work item id", id); err != nil {
		return nil, err
	}
	context := newValidationContext(baseDir)
	workItemPath, err := containedPath(filepath.Join(baseDir, ".sdd"), "work-items", "active", id)
	if err != nil {
		return nil, err
	}
	activeInfo, activeErr := os.Stat(workItemPath)
	activeExists := activeErr == nil && activeInfo.IsDir()
	if activeErr != nil && !os.IsNotExist(activeErr) {
		return nil, fmt.Errorf("inspect work item %s: %w", id, activeErr)
	}
	archiveTargets, err := inspector.archiveTargetsForID(baseDir, id)
	if err != nil {
		return nil, err
	}
	if !activeExists && len(archiveTargets) == 0 {
		return nil, domain.ErrWorkItemNotFound
	}

	inspector.inspectSchemas(context)
	if activeExists {
		activeTarget := filepath.ToSlash(filepath.Join(".sdd", "work-items", "active", id))
		inspector.inspectWorkItem(context, id, true, activeTarget, ports.WorkItemLocationActive)
	}
	for _, target := range archiveTargets {
		if activeExists || len(archiveTargets) > 1 {
			context.fail(
				categoryProject,
				"work_item.location_unique",
				target,
				fmt.Sprintf("work item %s has multiple storage locations", id),
			)
		}
		inspector.inspectWorkItem(context, id, true, target, ports.WorkItemLocationArchive)
	}
	return context.checks, nil
}

func (inspector *FSValidationInspector) archiveTargetsForID(baseDir, id string) ([]string, error) {
	archiveRoot, err := containedPath(filepath.Join(baseDir, ".sdd"), "work-items", "archive")
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(archiveRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("inspect archive directory: %w", err)
	}
	var targets []string
	for _, entry := range entries {
		entryID, parseErr := parseArchiveDirectoryName(entry.Name())
		if parseErr != nil || entryID != id || !entry.IsDir() {
			continue
		}
		targets = append(targets, filepath.ToSlash(filepath.Join(".sdd", "work-items", "archive", entry.Name())))
	}
	return targets, nil
}

func newValidationContext(baseDir string) *validationContext {
	return &validationContext{
		baseDir:         baseDir,
		schemaAvailable: make(map[string]bool),
		workflows:       make(map[string]*domain.Workflow),
	}
}

func (context *validationContext) add(status domain.CheckStatus, category, code, target, message string) {
	context.checks = append(context.checks, domain.ValidationCheck{
		Status:   status,
		Category: category,
		Code:     code,
		Target:   filepath.ToSlash(target),
		Message:  message,
	})
}

func (context *validationContext) pass(category, code, target, message string) {
	context.add(domain.CheckPassed, category, code, target, message)
}

func (context *validationContext) warn(category, code, target, message string) {
	context.add(domain.CheckWarning, category, code, target, message)
}

func (context *validationContext) fail(category, code, target, message string) {
	context.add(domain.CheckFailed, category, code, target, message)
}

func (context *validationContext) absolute(relative string) string {
	return filepath.Join(context.baseDir, filepath.FromSlash(relative))
}

func (context *validationContext) inspectDirectory(relative, category, code string, required bool) bool {
	info, err := os.Stat(context.absolute(relative))
	if err != nil {
		if required || !os.IsNotExist(err) {
			context.fail(category, code, relative, err.Error())
		}
		return false
	}
	if !info.IsDir() {
		context.fail(category, "project.required_path_type", relative, "path must be a directory")
		return false
	}
	context.pass(category, code, relative, "directory is available")
	return true
}

func (context *validationContext) inspectRegularFile(relative, category, code string, required bool) bool {
	info, err := os.Stat(context.absolute(relative))
	if err != nil {
		if required || !os.IsNotExist(err) {
			context.fail(category, code, relative, err.Error())
		}
		return false
	}
	if !info.Mode().IsRegular() {
		context.fail(category, code, relative, "path must be a regular file")
		return false
	}
	context.pass(category, code, relative, "file is available")
	return true
}

func (inspector *FSValidationInspector) inspectSchemas(context *validationContext) {
	for _, schemaFile := range requiredSchemas {
		target := filepath.ToSlash(filepath.Join(".sdd", "schemas", schemaFile))
		if !context.inspectRegularFile(target, categorySchema, "schema.file_exists", true) {
			context.schemaAvailable[schemaFile] = false
			continue
		}
		if err := inspector.schemas.Compile(context.baseDir, schemaFile); err != nil {
			context.fail(categorySchema, "schema.compiles", target, err.Error())
			context.schemaAvailable[schemaFile] = false
			continue
		}
		context.pass(categorySchema, "schema.compiles", target, "JSON Schema compiles")
		context.schemaAvailable[schemaFile] = true
	}
}

func (inspector *FSValidationInspector) inspectConfig(context *validationContext) string {
	const target = ".sdd/config.yaml"
	data, err := os.ReadFile(context.absolute(target))
	if err != nil {
		context.fail(categoryConfig, "config.yaml_valid", target, err.Error())
		return ""
	}
	var config validationConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		context.fail(categoryConfig, "config.yaml_valid", target, err.Error())
		return ""
	}
	context.pass(categoryConfig, "config.yaml_valid", target, "configuration is valid YAML")
	if config.SchemaVersion != "0.1" {
		context.fail(categoryConfig, "config.schema_version_supported", target, fmt.Sprintf("unsupported schema version %q", config.SchemaVersion))
	} else {
		context.pass(categoryConfig, "config.schema_version_supported", target, "schema version 0.1 is supported")
	}
	if err := domain.ValidateIdentifier("default workflow", config.Defaults.Workflow); err != nil {
		context.fail(categoryConfig, "config.default_workflow_valid", target, err.Error())
		return ""
	}
	context.pass(categoryConfig, "config.default_workflow_valid", target, fmt.Sprintf("default workflow %q is valid", config.Defaults.Workflow))
	return config.Defaults.Workflow
}

func (inspector *FSValidationInspector) inspectWorkflows(context *validationContext, onlyID string) {
	workflowsPath := context.absolute(".sdd/workflows")
	entries, err := os.ReadDir(workflowsPath)
	if err != nil {
		context.fail(categoryWorkflow, "workflow.directory_readable", ".sdd/workflows", err.Error())
		return
	}
	found := false
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".workflow.yaml") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".workflow.yaml")
		if onlyID != "" && id != onlyID {
			continue
		}
		found = true
		if workflow := inspector.inspectWorkflowFile(context, id); workflow != nil {
			context.workflows[workflow.ID] = workflow
		}
	}
	if onlyID == "" && !found {
		context.fail(categoryWorkflow, "workflow.files_present", ".sdd/workflows", "no workflow files found")
	}
	if onlyID != "" && !found {
		target := filepath.ToSlash(filepath.Join(".sdd", "workflows", onlyID+".workflow.yaml"))
		context.fail(categoryWorkflow, "workflow.file_exists", target, "referenced workflow file does not exist")
	}
}

func (inspector *FSValidationInspector) inspectWorkflowFile(context *validationContext, filenameID string) *domain.Workflow {
	target := filepath.ToSlash(filepath.Join(".sdd", "workflows", filenameID+".workflow.yaml"))
	data, err := os.ReadFile(context.absolute(target))
	if err != nil {
		context.fail(categoryWorkflow, "workflow.yaml_valid", target, err.Error())
		return nil
	}
	var workflow domain.Workflow
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		context.fail(categoryWorkflow, "workflow.yaml_valid", target, err.Error())
		return nil
	}
	context.pass(categoryWorkflow, "workflow.yaml_valid", target, "workflow is valid YAML")

	if context.schemaAvailable["workflow.schema.json"] {
		violations, err := inspector.schemas.ValidateYAMLAll(context.baseDir, "workflow.schema.json", data)
		if err != nil {
			context.fail(categoryWorkflow, "workflow.schema_valid", target, err.Error())
		} else if len(violations) == 0 {
			context.pass(categoryWorkflow, "workflow.schema_valid", target, "workflow matches workflow.schema.json")
		} else {
			for _, violation := range violations {
				context.fail(categoryWorkflow, "workflow.schema_valid", target, schemaViolationMessage(violation))
			}
		}
	}

	if workflow.ID != filenameID {
		context.fail(categoryWorkflow, "workflow.filename_matches_id", target, fmt.Sprintf("workflow id %q does not match filename %q", workflow.ID, filenameID))
	} else {
		context.pass(categoryWorkflow, "workflow.filename_matches_id", target, "workflow id matches filename")
	}

	semanticViolations := workflow.SemanticViolations()
	if len(semanticViolations) == 0 {
		context.pass(categoryWorkflow, "workflow.semantics_valid", target, "workflow semantics are valid")
	} else {
		for _, violation := range semanticViolations {
			context.fail(categoryWorkflow, violation.Code, target, violation.Message)
		}
	}

	inspector.inspectWorkflowResources(context, &workflow, target)
	return &workflow
}

func (inspector *FSValidationInspector) inspectWorkflowResources(
	context *validationContext,
	workflow *domain.Workflow,
	workflowTarget string,
) {
	artifactIDs := make([]string, 0, len(workflow.Artifacts))
	for artifactID := range workflow.Artifacts {
		artifactIDs = append(artifactIDs, artifactID)
	}
	sort.Strings(artifactIDs)
	for _, artifactID := range artifactIDs {
		artifact := workflow.Artifacts[artifactID]
		target := filepath.ToSlash(filepath.Join(".sdd", "templates", artifact.Template+".md"))
		if err := domain.ValidateIdentifier("template id", artifact.Template); err != nil {
			context.fail(categoryTemplate, "template.file_exists", workflowTarget, err.Error())
			continue
		}
		data, err := os.ReadFile(context.absolute(target))
		if err != nil {
			context.fail(categoryTemplate, "template.file_exists", target, err.Error())
			continue
		}
		context.pass(categoryTemplate, "template.file_exists", target, fmt.Sprintf("template for artifact %s is available", artifactID))

		phaseID, phase, exists := producerForArtifact(workflow, artifactID)
		if !exists {
			continue
		}
		rendered := domain.RenderTemplate(string(data), map[string]string{
			"title":           "Template validation",
			"id":              "template-validation",
			"type":            workflow.WorkItemType,
			"artifact_id":     artifactID,
			"phase":           phaseID,
			"created_at":      "2026-08-18T00:00:00Z",
			"created_by_kind": string(domain.ActorCLI),
			"created_by_id":   "sdd",
			"sources":         artifactSources(workflow, phase),
		})
		if strings.Contains(rendered, "{{") {
			context.fail(categoryTemplate, "template.placeholders_resolved", target, "template contains unresolved placeholders")
			continue
		}
		context.pass(categoryTemplate, "template.placeholders_resolved", target, "template placeholders resolve")
		metadata, err := extractFrontMatter(rendered)
		if err != nil {
			context.fail(categoryTemplate, "template.front_matter_valid", target, err.Error())
			continue
		}
		if context.schemaAvailable["artifact.schema.json"] {
			violations, validateErr := inspector.schemas.ValidateYAMLAll(context.baseDir, "artifact.schema.json", metadata)
			if validateErr != nil {
				context.fail(categoryTemplate, "template.front_matter_valid", target, validateErr.Error())
			} else if len(violations) > 0 {
				for _, violation := range violations {
					context.fail(categoryTemplate, "template.front_matter_valid", target, schemaViolationMessage(violation))
				}
			} else {
				context.pass(categoryTemplate, "template.front_matter_valid", target, "rendered front matter matches artifact schema")
			}
		}
		if err := validateArtifactMetadata(metadata, artifactID, phaseID, "template-validation"); err != nil {
			context.fail(categoryTemplate, "template.identity_valid", target, err.Error())
		} else {
			context.pass(categoryTemplate, "template.identity_valid", target, "rendered identity is valid")
		}
	}

	seenProcedures := make(map[string]struct{})
	for _, phase := range workflow.Phases {
		if phase.Procedure == "" {
			continue
		}
		if _, seen := seenProcedures[phase.Procedure]; seen {
			continue
		}
		seenProcedures[phase.Procedure] = struct{}{}
		target := filepath.ToSlash(filepath.Join(".sdd", "procedures", phase.Procedure+".md"))
		if err := domain.ValidateIdentifier("procedure id", phase.Procedure); err != nil {
			context.fail(categoryProcedure, "procedure.file_exists", workflowTarget, err.Error())
			continue
		}
		if context.inspectRegularFile(target, categoryProcedure, "procedure.file_exists", true) {
			context.pass(categoryProcedure, "procedure.file_regular", target, "procedure is a regular file")
		}
	}
}

func (inspector *FSValidationInspector) inspectWorkItem(
	context *validationContext,
	id string,
	inspectReferencedWorkflow bool,
	baseTarget string,
	location ports.WorkItemLocation,
) {
	manifestTarget := filepath.ToSlash(filepath.Join(baseTarget, "manifest.yaml"))
	data, err := os.ReadFile(context.absolute(manifestTarget))
	if err != nil {
		context.fail(categoryManifest, "manifest.file_exists", manifestTarget, err.Error())
		return
	}
	context.pass(categoryManifest, "manifest.file_exists", manifestTarget, "manifest is available")

	var item domain.WorkItem
	if err := yaml.Unmarshal(data, &item); err != nil {
		context.fail(categoryManifest, "manifest.yaml_valid", manifestTarget, err.Error())
		return
	}
	context.pass(categoryManifest, "manifest.yaml_valid", manifestTarget, "manifest is valid YAML")
	if context.schemaAvailable["work-item.schema.json"] {
		violations, validateErr := inspector.schemas.ValidateYAMLAll(context.baseDir, "work-item.schema.json", data)
		if validateErr != nil {
			context.fail(categoryManifest, "manifest.schema_valid", manifestTarget, validateErr.Error())
		} else if len(violations) == 0 {
			context.pass(categoryManifest, "manifest.schema_valid", manifestTarget, "manifest matches work-item.schema.json")
		} else {
			for _, violation := range violations {
				context.fail(categoryManifest, "manifest.schema_valid", manifestTarget, schemaViolationMessage(violation))
			}
		}
	}
	if item.ID != id {
		context.fail(categoryManifest, "manifest.id_matches_directory", manifestTarget, fmt.Sprintf("manifest id %q does not match work item id %q", item.ID, id))
	} else {
		context.pass(categoryManifest, "manifest.id_matches_directory", manifestTarget, "manifest id matches directory")
	}
	if location == ports.WorkItemLocationArchive && item.Status != domain.WorkItemArchived {
		context.fail(categoryManifest, "archive.manifest_status_matches_location", manifestTarget, fmt.Sprintf("archived work item has status %s", item.Status))
	} else if location == ports.WorkItemLocationActive && item.Status == domain.WorkItemArchived {
		context.fail(categoryManifest, "archive.manifest_status_matches_location", manifestTarget, "active work item cannot have archived status")
	} else {
		context.pass(categoryManifest, "archive.manifest_status_matches_location", manifestTarget, "manifest status matches storage location")
	}

	if inspectReferencedWorkflow {
		inspector.inspectWorkflows(context, item.Workflow.ID)
	}
	workflow, exists := context.workflows[item.Workflow.ID]
	if !exists {
		context.fail(categoryWorkItem, "work_item.workflow_available", manifestTarget, fmt.Sprintf("workflow %q is unavailable or invalid", item.Workflow.ID))
		inspector.inspectExternalReference(context, &item, manifestTarget)
		return
	}
	context.pass(categoryWorkItem, "work_item.workflow_available", manifestTarget, fmt.Sprintf("workflow %q is available", item.Workflow.ID))
	semanticViolations := item.ViolationsAgainst(workflow)
	if len(semanticViolations) == 0 {
		context.pass(categoryWorkItem, "work_item.semantics_valid", manifestTarget, "work item semantics are valid")
	} else {
		for _, violation := range semanticViolations {
			context.fail(categoryWorkItem, violation.Code, manifestTarget, violation.Message)
		}
	}

	inspector.inspectArtifacts(context, &item, workflow, baseTarget)
	inspector.inspectEvents(context, &item, workflow, baseTarget)
	inspector.inspectReferences(context, &item, manifestTarget)
}

func (inspector *FSValidationInspector) inspectArtifacts(
	context *validationContext,
	item *domain.WorkItem,
	workflow *domain.Workflow,
	baseTarget string,
) {
	artifactPaths := make(map[string]string, len(workflow.Artifacts))
	producerStates := make(map[string]domain.PhaseStatus, len(workflow.Artifacts))
	for artifactID, artifact := range workflow.Artifacts {
		artifactPaths[filepath.ToSlash(artifact.Path)] = artifactID
		phaseID, _, exists := producerForArtifact(workflow, artifactID)
		if exists {
			producerStates[filepath.ToSlash(artifact.Path)] = item.Phases[phaseID].Status
		}
	}

	for _, phase := range workflow.Phases {
		state, exists := item.Phases[phase.ID]
		if !exists {
			continue
		}
		if !artifactRequired(state.Status) {
			continue
		}
		artifactID := ""
		if len(phase.Produces) > 0 {
			artifactID = phase.Produces[0]
		}
		artifactConfig, exists := workflow.Artifacts[artifactID]
		if !exists {
			continue
		}
		target := filepath.ToSlash(filepath.Join(baseTarget, artifactConfig.Path))
		data, err := os.ReadFile(context.absolute(target))
		if err != nil {
			context.fail(categoryArtifact, "artifact.required_exists", target, err.Error())
			continue
		}
		context.pass(categoryArtifact, "artifact.required_exists", target, "required artifact is available")
		metadata, err := extractFrontMatter(string(data))
		if err != nil {
			context.fail(categoryArtifact, "artifact.front_matter_present", target, err.Error())
			continue
		}
		context.pass(categoryArtifact, "artifact.front_matter_present", target, "artifact has YAML front matter")
		if context.schemaAvailable["artifact.schema.json"] {
			violations, validateErr := inspector.schemas.ValidateYAMLAll(context.baseDir, "artifact.schema.json", metadata)
			if validateErr != nil {
				context.fail(categoryArtifact, "artifact.front_matter_schema_valid", target, validateErr.Error())
			} else if len(violations) == 0 {
				context.pass(categoryArtifact, "artifact.front_matter_schema_valid", target, "front matter matches artifact schema")
			} else {
				for _, violation := range violations {
					context.fail(categoryArtifact, "artifact.front_matter_schema_valid", target, schemaViolationMessage(violation))
				}
			}
		}
		if err := validateArtifactMetadata(metadata, artifactID, phase.ID, item.ID); err != nil {
			context.fail(categoryArtifact, "artifact.identity_valid", target, err.Error())
		} else {
			context.pass(categoryArtifact, "artifact.identity_valid", target, "artifact identity matches manifest and workflow")
		}

		var frontMatter struct {
			Sources []string `yaml:"sources"`
		}
		if err := yaml.Unmarshal(metadata, &frontMatter); err != nil {
			continue
		}
		for _, source := range frontMatter.Sources {
			source = filepath.ToSlash(source)
			sourceID, declared := artifactPaths[source]
			if !declared {
				context.fail(categoryArtifact, "artifact.source_declared", target, fmt.Sprintf("source %q is not declared by workflow", source))
				continue
			}
			context.pass(categoryArtifact, "artifact.source_declared", target, fmt.Sprintf("source %q maps to artifact %s", source, sourceID))
			if producerStates[source] == domain.PhaseNotApplicable {
				continue
			}
			sourceTarget := filepath.ToSlash(filepath.Join(baseTarget, source))
			if info, statErr := os.Stat(context.absolute(sourceTarget)); statErr != nil || !info.Mode().IsRegular() {
				message := "source artifact is unavailable"
				if statErr != nil {
					message = statErr.Error()
				}
				context.fail(categoryArtifact, "artifact.source_available", sourceTarget, message)
			} else {
				context.pass(categoryArtifact, "artifact.source_available", sourceTarget, "source artifact is available")
			}
		}
	}
}

func (inspector *FSValidationInspector) inspectEvents(
	context *validationContext,
	item *domain.WorkItem,
	workflow *domain.Workflow,
	baseTarget string,
) {
	target := filepath.ToSlash(filepath.Join(baseTarget, "events.jsonl"))
	file, err := os.Open(context.absolute(target))
	if err != nil {
		context.fail(categoryEvent, "event.file_exists", target, err.Error())
		return
	}
	defer file.Close()
	context.pass(categoryEvent, "event.file_exists", target, "event log is available")

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	lineNumber := 0
	eventCount := 0
	seenIDs := make(map[string]struct{})
	lastTransitions := make(map[string]domain.PhaseStatus)
	lifecycleStatus := domain.WorkItemActive
	archiveEvents := 0
	for scanner.Scan() {
		lineNumber++
		lineTarget := fmt.Sprintf("%s:%d", target, lineNumber)
		if len(strings.TrimSpace(scanner.Text())) == 0 {
			context.fail(categoryEvent, "event.line_json_valid", lineTarget, "JSONL line must not be empty")
			continue
		}
		var event domain.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			context.fail(categoryEvent, "event.line_json_valid", lineTarget, err.Error())
			continue
		}
		eventCount++
		context.pass(categoryEvent, "event.line_json_valid", lineTarget, "event is valid JSON")
		if context.schemaAvailable["event.schema.json"] {
			violations, validateErr := inspector.schemas.ValidateValueAll(context.baseDir, "event.schema.json", event)
			if validateErr != nil {
				context.fail(categoryEvent, "event.schema_valid", lineTarget, validateErr.Error())
			} else if len(violations) == 0 {
				context.pass(categoryEvent, "event.schema_valid", lineTarget, "event matches event schema")
			} else {
				for _, violation := range violations {
					context.fail(categoryEvent, "event.schema_valid", lineTarget, schemaViolationMessage(violation))
				}
			}
		}
		if eventCount == 1 {
			if event.Type == "work_item.created" {
				context.pass(categoryEvent, "event.first_is_creation", lineTarget, "first event records work item creation")
			} else {
				context.fail(categoryEvent, "event.first_is_creation", lineTarget, fmt.Sprintf("first event type is %q", event.Type))
			}
		}
		if event.WorkItem != item.ID {
			context.fail(categoryEvent, "event.work_item_matches", lineTarget, fmt.Sprintf("event work item %q does not match %q", event.WorkItem, item.ID))
		} else {
			context.pass(categoryEvent, "event.work_item_matches", lineTarget, "event work item matches manifest")
		}
		if _, exists := seenIDs[event.ID]; exists {
			context.fail(categoryEvent, "event.id_unique", lineTarget, fmt.Sprintf("event id %q is duplicated", event.ID))
		} else {
			seenIDs[event.ID] = struct{}{}
			context.pass(categoryEvent, "event.id_unique", lineTarget, "event id is unique")
		}
		if err := domain.ValidateOperationID(event.CorrelationID); err != nil {
			context.fail(categoryEvent, "event.correlation_id_valid", lineTarget, err.Error())
		} else {
			context.pass(categoryEvent, "event.correlation_id_valid", lineTarget, "correlation id is valid")
		}
		if event.Type == "phase.transitioned" {
			inspector.inspectTransitionEvent(context, event, workflow, lineTarget, lastTransitions)
		}
		if event.Type == "approval.requested" || event.Type == "approval.recorded" {
			inspector.inspectApprovalEvent(context, event, workflow, lineTarget)
		}
		switch event.Type {
		case "work_item.completed":
			if lifecycleStatus != domain.WorkItemActive {
				context.fail(categoryEvent, "event.lifecycle_continuity_valid", lineTarget, fmt.Sprintf("work item completion follows %s", lifecycleStatus))
			} else {
				context.pass(categoryEvent, "event.lifecycle_continuity_valid", lineTarget, "work item completion follows active state")
			}
			lifecycleStatus = domain.WorkItemCompleted
		case "archive.completed":
			archiveEvents++
			if lifecycleStatus != domain.WorkItemCompleted {
				context.fail(categoryEvent, "event.lifecycle_continuity_valid", lineTarget, fmt.Sprintf("archive completion follows %s", lifecycleStatus))
			} else {
				context.pass(categoryEvent, "event.lifecycle_continuity_valid", lineTarget, "archive completion follows completed state")
			}
			inspector.inspectArchiveEvent(context, event, baseTarget, lineTarget)
			lifecycleStatus = domain.WorkItemArchived
		}
	}
	if err := scanner.Err(); err != nil {
		context.fail(categoryEvent, "event.file_readable", target, err.Error())
	}
	for phaseID, status := range lastTransitions {
		if item.Phases[phaseID].Status != status {
			context.fail(
				categoryEvent,
				"event.transition_matches_manifest",
				target,
				fmt.Sprintf("last transition for phase %s ends in %s but manifest is %s", phaseID, status, item.Phases[phaseID].Status),
			)
		} else {
			context.pass(categoryEvent, "event.transition_matches_manifest", target, fmt.Sprintf("phase %s transition history matches manifest", phaseID))
		}
	}
	if archiveEvents > 1 {
		context.fail(categoryEvent, "event.archive_unique", target, fmt.Sprintf("event log contains %d archive.completed events", archiveEvents))
	} else {
		context.pass(categoryEvent, "event.archive_unique", target, "archive completion event is unique")
	}
	if item.Status != lifecycleStatus {
		context.fail(
			categoryEvent,
			"event.lifecycle_matches_manifest",
			target,
			fmt.Sprintf("event lifecycle ends in %s but manifest is %s", lifecycleStatus, item.Status),
		)
	} else {
		context.pass(categoryEvent, "event.lifecycle_matches_manifest", target, "event lifecycle matches manifest status")
	}
}

func (inspector *FSValidationInspector) inspectArchiveEvent(
	context *validationContext,
	event domain.Event,
	baseTarget, target string,
) {
	from, fromOK := event.Data["from"].(string)
	to, toOK := event.Data["to"].(string)
	archivePath, pathOK := event.Data["archive_path"].(string)
	if !fromOK || !toOK || !pathOK ||
		from != string(domain.WorkItemCompleted) ||
		to != string(domain.WorkItemArchived) ||
		filepath.ToSlash(archivePath) != filepath.ToSlash(baseTarget) {
		context.fail(categoryEvent, "event.archive_payload_valid", target, "archive event requires completed -> archived and the persisted archive path")
		return
	}
	context.pass(categoryEvent, "event.archive_payload_valid", target, "archive event payload matches archived location")
}

func (inspector *FSValidationInspector) inspectTransitionEvent(
	context *validationContext,
	event domain.Event,
	workflow *domain.Workflow,
	target string,
	lastTransitions map[string]domain.PhaseStatus,
) {
	phaseID, phaseOK := event.Data["phase"].(string)
	fromValue, fromOK := event.Data["from"].(string)
	toValue, toOK := event.Data["to"].(string)
	cause, causeOK := event.Data["cause"].(string)
	if !phaseOK || !fromOK || !toOK || !causeOK || cause == "" {
		context.fail(categoryEvent, "event.transition_payload_valid", target, "phase transition requires string phase, from, to, and cause")
		return
	}
	if _, exists := workflow.Phase(phaseID); !exists {
		context.fail(categoryEvent, "event.transition_payload_valid", target, fmt.Sprintf("transition references unknown phase %q", phaseID))
		return
	}
	from := domain.PhaseStatus(fromValue)
	to := domain.PhaseStatus(toValue)
	if !knownPhaseStatus(from) || !knownPhaseStatus(to) || from == to {
		context.fail(categoryEvent, "event.transition_payload_valid", target, fmt.Sprintf("invalid transition %s -> %s", from, to))
		return
	}
	context.pass(categoryEvent, "event.transition_payload_valid", target, fmt.Sprintf("transition %s %s -> %s is valid", phaseID, from, to))
	if previous, exists := lastTransitions[phaseID]; exists && previous != from {
		context.fail(categoryEvent, "event.transition_continuity_valid", target, fmt.Sprintf("phase %s previously ended in %s but transition starts in %s", phaseID, previous, from))
	} else {
		context.pass(categoryEvent, "event.transition_continuity_valid", target, fmt.Sprintf("phase %s transition is continuous", phaseID))
	}
	lastTransitions[phaseID] = to
}

func (inspector *FSValidationInspector) inspectApprovalEvent(
	context *validationContext,
	event domain.Event,
	workflow *domain.Workflow,
	target string,
) {
	phaseID, ok := event.Data["phase"].(string)
	phase, exists := workflow.Phase(phaseID)
	if !ok || !exists || phase.Approval == domain.ApprovalNone {
		context.fail(categoryEvent, "event.approval_payload_valid", target, "approval event references an invalid phase")
		return
	}
	if event.Type == "approval.recorded" && event.Actor.Kind != domain.ActorHuman {
		context.fail(categoryEvent, "event.approval_payload_valid", target, "recorded approval requires a human actor")
		return
	}
	context.pass(categoryEvent, "event.approval_payload_valid", target, "approval event payload is valid")
}

func (inspector *FSValidationInspector) inspectReferences(
	context *validationContext,
	item *domain.WorkItem,
	manifestTarget string,
) {
	inspector.inspectExternalReference(context, item, manifestTarget)
	for _, reference := range item.Traceability.RelatedWorkItems {
		if err := domain.ValidateIdentifier("related work item id", reference); err != nil {
			context.fail(categoryReference, "reference.related_work_item_id_valid", manifestTarget, err.Error())
		} else {
			context.pass(categoryReference, "reference.related_work_item_id_valid", manifestTarget, fmt.Sprintf("related work item %q has a valid id", reference))
		}
	}
	for _, reference := range item.Traceability.BaselineSpecs {
		clean := filepath.Clean(filepath.FromSlash(reference))
		if filepath.IsAbs(clean) || clean == "." || clean == ".." ||
			!strings.HasPrefix(filepath.ToSlash(clean), "specs/") {
			context.fail(categoryReference, "reference.baseline_spec_exists", manifestTarget, fmt.Sprintf("baseline spec %q must be inside specs/", reference))
			continue
		}
		target := filepath.ToSlash(filepath.Join(".sdd", clean))
		if context.inspectRegularFile(target, categoryReference, "reference.baseline_spec_exists", true) {
			context.pass(categoryReference, "reference.baseline_spec_exists", target, "baseline spec is available")
		}
	}
}

func (inspector *FSValidationInspector) inspectExternalReference(
	context *validationContext,
	item *domain.WorkItem,
	manifestTarget string,
) {
	if item.Input.ExternalArtifact == nil {
		return
	}
	reference := item.Input.ExternalArtifact
	if !filepath.IsAbs(reference.Path) {
		context.fail(categoryReference, "reference.external_regular", manifestTarget, "external artifact path must be absolute")
		return
	}
	info, err := os.Stat(reference.Path)
	if os.IsNotExist(err) {
		context.warn(categoryReference, "reference.external_available", manifestTarget, fmt.Sprintf("external source %q is not available on this machine", reference.Path))
		return
	}
	if err != nil {
		context.fail(categoryReference, "reference.external_available", manifestTarget, err.Error())
		return
	}
	context.pass(categoryReference, "reference.external_available", manifestTarget, "external source is available")
	if !info.Mode().IsRegular() {
		context.fail(categoryReference, "reference.external_regular", manifestTarget, "external source is not a regular file")
		return
	}
	context.pass(categoryReference, "reference.external_regular", manifestTarget, "external source is a regular file")
	data, err := os.ReadFile(reference.Path)
	if err != nil {
		context.fail(categoryReference, "reference.external_hash_matches", manifestTarget, err.Error())
		return
	}
	actual := fmt.Sprintf("%x", sha256.Sum256(data))
	if actual != reference.SHA256 {
		context.fail(categoryReference, "reference.external_hash_matches", manifestTarget, fmt.Sprintf("external source hash is %s, expected %s", actual, reference.SHA256))
	} else {
		context.pass(categoryReference, "reference.external_hash_matches", manifestTarget, "external source hash matches manifest")
	}
}

func producerForArtifact(workflow *domain.Workflow, artifactID string) (string, domain.WorkflowPhase, bool) {
	for _, phase := range workflow.Phases {
		for _, producedID := range phase.Produces {
			if producedID == artifactID {
				return phase.ID, phase, true
			}
		}
	}
	return "", domain.WorkflowPhase{}, false
}

func artifactRequired(status domain.PhaseStatus) bool {
	switch status {
	case domain.PhaseReady,
		domain.PhaseInProgress,
		domain.PhaseAwaitingApproval,
		domain.PhaseApproved,
		domain.PhaseRejected,
		domain.PhaseCompleted,
		domain.PhaseAccepted,
		domain.PhaseSuperseded:
		return true
	default:
		return false
	}
}

func knownPhaseStatus(status domain.PhaseStatus) bool {
	switch status {
	case domain.PhaseNotApplicable,
		domain.PhaseBlocked,
		domain.PhaseReady,
		domain.PhaseInProgress,
		domain.PhaseAwaitingApproval,
		domain.PhaseApproved,
		domain.PhaseCompleted,
		domain.PhaseRejected,
		domain.PhaseAccepted,
		domain.PhaseSuperseded:
		return true
	default:
		return false
	}
}

func schemaViolationMessage(violation SchemaViolation) string {
	if violation.InstancePath == "" {
		return violation.Message
	}
	return fmt.Sprintf("%s: %s", violation.InstancePath, violation.Message)
}

var _ ports.ValidationInspector = (*FSValidationInspector)(nil)
