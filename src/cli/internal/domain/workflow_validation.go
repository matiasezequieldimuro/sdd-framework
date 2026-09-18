package domain

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

func (workflow *Workflow) ValidateSemantics() error {
	violations := workflow.SemanticViolations()
	if len(violations) == 0 {
		return nil
	}
	return violations[0]
}

func (workflow *Workflow) SemanticViolations() []ContractViolation {
	var violations []ContractViolation
	if err := ValidateIdentifier("workflow id", workflow.ID); err != nil {
		violations = append(violations, NewContractViolation("workflow.identifier_invalid", err, "%v", err))
	}
	if len(workflow.Phases) == 0 {
		violations = append(violations, NewContractViolation("workflow.phases_missing", ErrInvalidWorkflow, "workflow has no phases"))
	}
	if len(workflow.EntryPoints) == 0 {
		violations = append(violations, NewContractViolation("workflow.entry_points_missing", ErrInvalidWorkflow, "workflow has no entry points"))
	}

	phaseIDs := make(map[string]struct{}, len(workflow.Phases))
	artifactPaths := make(map[string]string, len(workflow.Artifacts))
	producedArtifacts := make(map[string]string, len(workflow.Artifacts))
	for _, phase := range workflow.Phases {
		if err := ValidateIdentifier("phase id", phase.ID); err != nil {
			violations = append(violations, NewContractViolation("workflow.phase_identifier_invalid", err, "%v", err))
		}
		if _, exists := phaseIDs[phase.ID]; exists {
			violations = append(violations, NewContractViolation("workflow.phase_duplicate", ErrInvalidWorkflow, "duplicate phase id %q", phase.ID))
		}
		phaseIDs[phase.ID] = struct{}{}
		switch phase.Approval {
		case ApprovalNone, ApprovalRequired, ApprovalOptional:
		default:
			violations = append(violations, NewContractViolation(
				"workflow.approval_policy_invalid",
				ErrInvalidWorkflow,
				"phase %s has unknown approval policy %q",
				phase.ID,
				phase.Approval,
			))
		}
		if len(phase.Produces) != 1 {
			violations = append(violations, NewContractViolation(
				"workflow.phase_artifact_count_invalid",
				ErrInvalidWorkflow,
				"phase %s must produce exactly one artifact in contract v0.1",
				phase.ID,
			))
		}
	}

	artifactIDs := make([]string, 0, len(workflow.Artifacts))
	for artifactID := range workflow.Artifacts {
		artifactIDs = append(artifactIDs, artifactID)
	}
	sort.Strings(artifactIDs)
	for _, artifactID := range artifactIDs {
		artifact := workflow.Artifacts[artifactID]
		if err := ValidateIdentifier("artifact id", artifactID); err != nil {
			violations = append(violations, NewContractViolation("workflow.artifact_identifier_invalid", err, "%v", err))
		}
		if err := ValidateIdentifier("template id", artifact.Template); err != nil {
			violations = append(violations, NewContractViolation("workflow.template_identifier_invalid", err, "%v", err))
		}
		if err := validateArtifactPath(artifact.Path); err != nil {
			violations = append(violations, NewContractViolation(
				"workflow.artifact_path_invalid",
				ErrInvalidWorkflow,
				"artifact %s: %v",
				artifactID,
				err,
			))
		}
		if existing, exists := artifactPaths[artifact.Path]; exists {
			violations = append(violations, NewContractViolation(
				"workflow.artifact_path_duplicate",
				ErrInvalidWorkflow,
				"artifacts %s and %s share path %q",
				existing,
				artifactID,
				artifact.Path,
			))
		}
		artifactPaths[artifact.Path] = artifactID
	}

	for _, phase := range workflow.Phases {
		for _, requiredID := range phase.Requires {
			if requiredID == phase.ID {
				violations = append(violations, NewContractViolation(
					"workflow.phase_self_dependency",
					ErrInvalidWorkflow,
					"phase %s requires itself",
					phase.ID,
				))
			}
			if _, exists := phaseIDs[requiredID]; !exists {
				violations = append(violations, NewContractViolation(
					"workflow.phase_dependency_unknown",
					ErrInvalidWorkflow,
					"phase %s requires unknown phase %s",
					phase.ID,
					requiredID,
				))
			}
		}
		for _, artifactID := range phase.Produces {
			if _, exists := workflow.Artifacts[artifactID]; !exists {
				violations = append(violations, NewContractViolation(
					"workflow.produced_artifact_unknown",
					ErrInvalidWorkflow,
					"phase %s produces unknown artifact %s",
					phase.ID,
					artifactID,
				))
			}
			if producer, exists := producedArtifacts[artifactID]; exists {
				violations = append(violations, NewContractViolation(
					"workflow.artifact_producer_duplicate",
					ErrInvalidWorkflow,
					"phases %s and %s both produce artifact %s",
					producer,
					phase.ID,
					artifactID,
				))
			}
			producedArtifacts[artifactID] = phase.ID
		}
	}
	for _, artifactID := range artifactIDs {
		if _, exists := producedArtifacts[artifactID]; !exists {
			violations = append(violations, NewContractViolation(
				"workflow.artifact_producer_missing",
				ErrInvalidWorkflow,
				"artifact %s is not produced by any phase",
				artifactID,
			))
		}
	}

	seenEntries := make(map[string]struct{}, len(workflow.EntryPoints))
	inputEntries := make(map[string]string)
	for _, entry := range workflow.EntryPoints {
		phase, exists := workflow.Phase(entry.Phase)
		if !exists {
			violations = append(violations, NewContractViolation(
				"workflow.entry_phase_unknown",
				ErrInvalidWorkflow,
				"entry point references unknown phase %s",
				entry.Phase,
			))
			continue
		}
		if _, duplicate := seenEntries[entry.Phase]; duplicate {
			violations = append(violations, NewContractViolation(
				"workflow.entry_phase_duplicate",
				ErrInvalidWorkflow,
				"duplicate entry point for phase %s",
				entry.Phase,
			))
		}
		seenEntries[entry.Phase] = struct{}{}

		produced := make(map[string]struct{}, len(phase.Produces))
		for _, artifactID := range phase.Produces {
			produced[artifactID] = struct{}{}
		}
		for _, acceptedInput := range entry.Accepts {
			if existingPhase, exists := inputEntries[acceptedInput]; exists && existingPhase != entry.Phase {
				violations = append(violations, NewContractViolation(
					"workflow.entry_input_ambiguous",
					ErrInvalidWorkflow,
					"input %s is accepted by both %s and %s",
					acceptedInput,
					existingPhase,
					entry.Phase,
				))
			}
			inputEntries[acceptedInput] = entry.Phase
			if acceptedInput == "user_prompt" {
				continue
			}
			if _, exists := produced[acceptedInput]; !exists {
				violations = append(violations, NewContractViolation(
					"workflow.entry_input_unknown",
					ErrInvalidWorkflow,
					"entry point %s accepts unknown input %s",
					entry.Phase,
					acceptedInput,
				))
			}
		}
	}

	ordered, err := workflow.OrderedPhases()
	if err != nil {
		violations = append(violations, NewContractViolation("workflow.graph_cycle", ErrInvalidWorkflow, "%v", err))
	} else {
		reachable := workflow.reachableFromEntries()
		if len(reachable) != len(ordered) {
			for _, phase := range ordered {
				if _, exists := reachable[phase.ID]; !exists {
					violations = append(violations, NewContractViolation(
						"workflow.phase_unreachable",
						ErrInvalidWorkflow,
						"phase %s is unreachable from every entry point",
						phase.ID,
					))
				}
			}
		}
	}

	return violations
}

func (workflow *Workflow) EntryPhaseFor(input string) (string, error) {
	var matches []string
	for _, entry := range workflow.EntryPoints {
		for _, accepted := range entry.Accepts {
			if accepted == input {
				matches = append(matches, entry.Phase)
				break
			}
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("%w: workflow %s has no entry point accepting %s", ErrInvalidEntryPoint, workflow.ID, input)
	}
	if len(matches) > 1 {
		sort.Strings(matches)
		return "", fmt.Errorf("%w: workflow %s has ambiguous entry points for %s: %s", ErrInvalidEntryPoint, workflow.ID, input, strings.Join(matches, ", "))
	}
	return matches[0], nil
}

func (workflow *Workflow) ExternalArtifactForEntry(phaseID string) (string, error) {
	phase, exists := workflow.Phase(phaseID)
	if !exists {
		return "", ErrPhaseNotFound
	}

	produced := make(map[string]struct{}, len(phase.Produces))
	for _, artifactID := range phase.Produces {
		produced[artifactID] = struct{}{}
	}

	var matches []string
	for _, entry := range workflow.EntryPoints {
		if entry.Phase != phaseID {
			continue
		}
		for _, accepted := range entry.Accepts {
			if accepted == "user_prompt" {
				continue
			}
			if _, exists := produced[accepted]; exists {
				matches = append(matches, accepted)
			}
		}
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("%w: phase %s must accept exactly one produced artifact, found %d", ErrInvalidEntryPoint, phaseID, len(matches))
	}
	return matches[0], nil
}

func (workflow *Workflow) OrderedPhases() ([]WorkflowPhase, error) {
	indegree := make(map[string]int, len(workflow.Phases))
	children := make(map[string][]string, len(workflow.Phases))
	phases := make(map[string]WorkflowPhase, len(workflow.Phases))
	for _, phase := range workflow.Phases {
		phases[phase.ID] = phase
		indegree[phase.ID] = len(phase.Requires)
		for _, requiredID := range phase.Requires {
			children[requiredID] = append(children[requiredID], phase.ID)
		}
	}

	var ready []string
	for phaseID, degree := range indegree {
		if degree == 0 {
			ready = append(ready, phaseID)
		}
	}
	sort.Strings(ready)

	ordered := make([]WorkflowPhase, 0, len(workflow.Phases))
	for len(ready) > 0 {
		phaseID := ready[0]
		ready = ready[1:]
		ordered = append(ordered, phases[phaseID])

		sort.Strings(children[phaseID])
		for _, childID := range children[phaseID] {
			indegree[childID]--
			if indegree[childID] == 0 {
				ready = append(ready, childID)
				sort.Strings(ready)
			}
		}
	}
	if len(ordered) != len(workflow.Phases) {
		return nil, fmt.Errorf("%w: workflow graph contains a cycle", ErrInvalidWorkflow)
	}

	return ordered, nil
}

func (workflow *Workflow) Ancestors(phaseID string) (map[string]struct{}, error) {
	if _, exists := workflow.Phase(phaseID); !exists {
		return nil, ErrPhaseNotFound
	}

	ancestors := make(map[string]struct{})
	var visit func(string)
	visit = func(currentID string) {
		phase, _ := workflow.Phase(currentID)
		for _, requiredID := range phase.Requires {
			if _, seen := ancestors[requiredID]; seen {
				continue
			}
			ancestors[requiredID] = struct{}{}
			visit(requiredID)
		}
	}
	visit(phaseID)

	return ancestors, nil
}

func (workflow *Workflow) reachableFromEntries() map[string]struct{} {
	children := make(map[string][]string, len(workflow.Phases))
	for _, phase := range workflow.Phases {
		for _, requiredID := range phase.Requires {
			children[requiredID] = append(children[requiredID], phase.ID)
		}
	}

	reachable := make(map[string]struct{})
	queue := make([]string, 0, len(workflow.EntryPoints))
	for _, entry := range workflow.EntryPoints {
		if _, seen := reachable[entry.Phase]; !seen {
			reachable[entry.Phase] = struct{}{}
			queue = append(queue, entry.Phase)
		}
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, childID := range children[current] {
			if _, seen := reachable[childID]; seen {
				continue
			}
			reachable[childID] = struct{}{}
			queue = append(queue, childID)
		}
	}

	return reachable
}

func validateArtifactPath(path string) error {
	if path == "" || filepath.IsAbs(path) {
		return fmt.Errorf("path %q must be a non-empty relative path", path)
	}
	clean := filepath.Clean(path)
	if clean != path || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q is not normalized or escapes its root", path)
	}
	if clean == "artifacts" || !strings.HasPrefix(clean, "artifacts"+string(filepath.Separator)) {
		return fmt.Errorf("path %q must be inside artifacts/", path)
	}
	if filepath.Ext(clean) != ".md" {
		return fmt.Errorf("path %q must reference a Markdown file", path)
	}
	return nil
}
