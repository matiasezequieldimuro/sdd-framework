package domain

import (
	"fmt"
	"time"
)

func (item *WorkItem) ValidateAgainst(workflow *Workflow) error {
	violations := item.ViolationsAgainst(workflow)
	if len(violations) == 0 {
		return nil
	}
	return violations[0]
}

func (item *WorkItem) ViolationsAgainst(workflow *Workflow) []ContractViolation {
	var violations []ContractViolation
	if workflow == nil {
		return []ContractViolation{NewContractViolation("work_item.workflow_missing", ErrInvalidWorkItem, "workflow is required")}
	}
	if item.Workflow.ID != workflow.ID {
		violations = append(violations, NewContractViolation(
			"work_item.workflow_mismatch",
			ErrInvalidWorkItem,
			"manifest workflow %s does not match %s",
			item.Workflow.ID,
			workflow.ID,
		))
	}
	if item.Workflow.Version != workflow.SchemaVersion {
		violations = append(violations, NewContractViolation(
			"work_item.workflow_version_mismatch",
			ErrInvalidWorkItem,
			"workflow version %s does not match %s",
			item.Workflow.Version,
			workflow.SchemaVersion,
		))
	}
	if item.Type != workflow.WorkItemType {
		violations = append(violations, NewContractViolation(
			"work_item.type_mismatch",
			ErrInvalidWorkItem,
			"work item type %s does not match workflow type %s",
			item.Type,
			workflow.WorkItemType,
		))
	}
	entryValid := workflow.hasEntryPhase(item.Workflow.EntryPhase)
	if !entryValid {
		violations = append(violations, NewContractViolation(
			"work_item.entry_point_invalid",
			ErrInvalidEntryPoint,
			"%s is not a declared entry point",
			item.Workflow.EntryPhase,
		))
	}
	if item.Input.Source == "external_artifact" {
		if item.Input.ExternalArtifact == nil {
			violations = append(violations, NewContractViolation(
				"work_item.external_artifact_missing",
				ErrInvalidExternalArtifact,
				"external artifact metadata is required",
			))
		} else if entryValid {
			acceptedArtifact, err := workflow.ExternalArtifactForEntry(item.Workflow.EntryPhase)
			if err != nil {
				violations = append(violations, NewContractViolation("work_item.external_entry_invalid", err, "%v", err))
			} else if item.Input.ExternalArtifact.Artifact != acceptedArtifact {
				violations = append(violations, NewContractViolation(
					"work_item.external_artifact_mismatch",
					ErrInvalidExternalArtifact,
					"entry phase %s accepts %s, got %s",
					item.Workflow.EntryPhase,
					acceptedArtifact,
					item.Input.ExternalArtifact.Artifact,
				))
			}
		}
	} else if item.Input.ExternalArtifact != nil {
		violations = append(violations, NewContractViolation(
			"work_item.external_artifact_unexpected",
			ErrInvalidExternalArtifact,
			"external artifact metadata is only valid for external input",
		))
	}
	if len(item.Phases) != len(workflow.Phases) {
		violations = append(violations, NewContractViolation(
			"work_item.phase_count_mismatch",
			ErrInvalidWorkItem,
			"manifest has %d phases, workflow requires %d",
			len(item.Phases),
			len(workflow.Phases),
		))
	}

	ancestors := make(map[string]struct{})
	if entryValid {
		var err error
		ancestors, err = workflow.Ancestors(item.Workflow.EntryPhase)
		if err != nil {
			violations = append(violations, NewContractViolation("work_item.entry_ancestors_invalid", err, "%v", err))
		}
	}
	for _, phase := range workflow.Phases {
		state, exists := item.Phases[phase.ID]
		if !exists {
			violations = append(violations, NewContractViolation(
				"work_item.phase_state_missing",
				ErrInvalidWorkItem,
				"phase %s has no state",
				phase.ID,
			))
			continue
		}
		if state.Artifact != "" && state.Artifact != workflow.ArtifactPathForPhase(phase.ID) {
			violations = append(violations, NewContractViolation(
				"work_item.artifact_path_mismatch",
				ErrInvalidWorkItem,
				"phase %s artifact path %q does not match workflow",
				phase.ID,
				state.Artifact,
			))
		}

		if state.Status == PhaseNotApplicable {
			if _, allowed := ancestors[phase.ID]; !allowed {
				violations = append(violations, NewContractViolation(
					"work_item.not_applicable_invalid",
					ErrInvalidWorkItem,
					"phase %s is not an ancestor of entry phase %s",
					phase.ID,
					item.Workflow.EntryPhase,
				))
			}
			continue
		}

		dependenciesSatisfied := item.dependenciesSatisfied(phase)
		isEntry := phase.ID == item.Workflow.EntryPhase
		if state.Status == PhaseBlocked {
			if dependenciesSatisfied && !isEntry && !phase.Optional {
				violations = append(violations, NewContractViolation(
					"work_item.phase_blocked_invalid",
					ErrInvalidWorkItem,
					"phase %s is blocked despite satisfied dependencies",
					phase.ID,
				))
			}
			continue
		}
		if !isEntry && !dependenciesSatisfied {
			violations = append(violations, NewContractViolation(
				"work_item.dependencies_unsatisfied",
				ErrInvalidWorkItem,
				"phase %s is %s before its dependencies are satisfied",
				phase.ID,
				state.Status,
			))
		}
		if (state.Status == PhaseAwaitingApproval || state.Status == PhaseApproved || state.Status == PhaseRejected) &&
			phase.Approval == ApprovalNone {
			violations = append(violations, NewContractViolation(
				"work_item.approval_state_invalid",
				ErrInvalidWorkItem,
				"phase %s uses approval state with policy none",
				phase.ID,
			))
		}
		if state.Status == PhaseAccepted && (isEntry == false || item.Input.Source == "user_prompt") {
			violations = append(violations, NewContractViolation(
				"work_item.accepted_state_invalid",
				ErrInvalidWorkItem,
				"phase %s cannot be accepted for input source %s",
				phase.ID,
				item.Input.Source,
			))
		}
	}
	for phaseID := range item.Phases {
		if _, exists := workflow.Phase(phaseID); !exists {
			violations = append(violations, NewContractViolation(
				"work_item.phase_unknown",
				ErrInvalidWorkItem,
				"manifest contains unknown phase %s",
				phaseID,
			))
		}
	}

	if item.Status == WorkItemCompleted || item.Status == WorkItemArchived {
		for _, phase := range workflow.Phases {
			state, exists := item.Phases[phase.ID]
			if !exists {
				continue
			}
			if item.Status == WorkItemCompleted && phase.Optional {
				continue
			}
			if phase.Optional && (state.Status == PhaseBlocked || state.Status == PhaseReady || state.Status == PhaseNotApplicable) {
				continue
			}
			if !state.Status.satisfiesCompletion() {
				violations = append(violations, NewContractViolation(
					"work_item.completion_invalid",
					ErrInvalidWorkItem,
					"completed work item has unsatisfied phase %s in %s",
					phase.ID,
					state.Status,
				))
			}
		}
	}
	if item.Status == WorkItemArchived {
		completed := *item
		completed.Status = WorkItemCompleted
		if err := completed.archiveEligibility(workflow); err != nil {
			violations = append(violations, NewContractViolation(
				"work_item.archive_invalid",
				ErrInvalidWorkItem,
				"%v",
				err,
			))
		}
	}

	return append(violations, item.approvalViolations(workflow)...)
}

func (item *WorkItem) approvalViolations(workflow *Workflow) []ContractViolation {
	var violations []ContractViolation
	latest := make(map[string]Approval)
	pending := make(map[string]int)

	for _, approval := range item.Approvals {
		phase, exists := workflow.Phase(approval.Phase)
		if !exists {
			violations = append(violations, NewContractViolation(
				"approval.phase_unknown",
				ErrInvalidWorkItem,
				"approval references unknown phase %s",
				approval.Phase,
			))
			continue
		}
		if phase.Approval == ApprovalNone {
			violations = append(violations, NewContractViolation(
				"approval.policy_invalid",
				ErrInvalidWorkItem,
				"phase %s does not allow approvals",
				approval.Phase,
			))
		}
		if approval.Status == ApprovalPending {
			pending[approval.Phase]++
			if approval.By != nil || approval.At != "" {
				violations = append(violations, NewContractViolation(
					"approval.pending_metadata_invalid",
					ErrInvalidWorkItem,
					"pending approval for phase %s must not contain decision metadata",
					approval.Phase,
				))
			}
		} else if approval.Status == ApprovalApproved || approval.Status == ApprovalRejected {
			if approval.By == nil || ValidateActor(*approval.By) != nil || approval.By.Kind != ActorHuman {
				violations = append(violations, NewContractViolation(
					"approval.actor_invalid",
					ErrInvalidWorkItem,
					"approval decision for phase %s requires a human actor",
					approval.Phase,
				))
			}
			if _, err := time.Parse(time.RFC3339Nano, approval.At); err != nil {
				violations = append(violations, NewContractViolation(
					"approval.timestamp_invalid",
					ErrInvalidWorkItem,
					"approval decision for phase %s has invalid timestamp %q",
					approval.Phase,
					approval.At,
				))
			}
		}
		latest[approval.Phase] = approval
	}

	for phaseID, count := range pending {
		if count > 1 {
			violations = append(violations, NewContractViolation(
				"approval.pending_duplicate",
				ErrInvalidWorkItem,
				"phase %s has %d pending approvals",
				phaseID,
				count,
			))
		}
	}

	for phaseID, state := range item.Phases {
		approval, exists := latest[phaseID]
		switch state.Status {
		case PhaseAwaitingApproval:
			if !exists || approval.Status != ApprovalPending {
				violations = append(violations, NewContractViolation(
					"approval.latest_mismatch",
					ErrInvalidWorkItem,
					"phase %s is awaiting approval without a latest pending approval",
					phaseID,
				))
			}
		case PhaseApproved:
			if !exists || approval.Status != ApprovalApproved {
				violations = append(violations, NewContractViolation(
					"approval.latest_mismatch",
					ErrInvalidWorkItem,
					"phase %s is approved without a latest approved decision",
					phaseID,
				))
			}
		case PhaseRejected:
			if !exists || approval.Status != ApprovalRejected {
				violations = append(violations, NewContractViolation(
					"approval.latest_mismatch",
					ErrInvalidWorkItem,
					"phase %s is rejected without a latest rejected decision",
					phaseID,
				))
			}
		}
	}

	return violations
}

func (item *WorkItem) validateCompletion(workflow *Workflow) error {
	for _, phase := range workflow.Phases {
		state := item.Phases[phase.ID]
		if phase.Optional && (state.Status == PhaseBlocked || state.Status == PhaseReady || state.Status == PhaseNotApplicable) {
			continue
		}
		if !state.Status.satisfiesCompletion() {
			return fmt.Errorf("%w: completed work item has unsatisfied phase %s in %s", ErrInvalidWorkItem, phase.ID, state.Status)
		}
	}
	return nil
}

func (workflow *Workflow) hasEntryPhase(phaseID string) bool {
	for _, entry := range workflow.EntryPoints {
		if entry.Phase == phaseID {
			return true
		}
	}
	return false
}
