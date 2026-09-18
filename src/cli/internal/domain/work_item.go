package domain

import "fmt"

type PhaseStatus string

const (
	PhaseNotApplicable    PhaseStatus = "not_applicable"
	PhaseBlocked          PhaseStatus = "blocked"
	PhaseReady            PhaseStatus = "ready"
	PhaseInProgress       PhaseStatus = "in_progress"
	PhaseAwaitingApproval PhaseStatus = "awaiting_approval"
	PhaseApproved         PhaseStatus = "approved"
	PhaseCompleted        PhaseStatus = "completed"
	PhaseRejected         PhaseStatus = "rejected"
	PhaseAccepted         PhaseStatus = "accepted"
	PhaseSuperseded       PhaseStatus = "superseded"
)

type WorkItemStatus string

const (
	WorkItemActive    WorkItemStatus = "active"
	WorkItemCompleted WorkItemStatus = "completed"
	WorkItemArchived  WorkItemStatus = "archived"
	WorkItemCancelled WorkItemStatus = "cancelled"
)

type ApprovalStatus string

const (
	ApprovalPending    ApprovalStatus = "pending"
	ApprovalApproved   ApprovalStatus = "approved"
	ApprovalRejected   ApprovalStatus = "rejected"
	ApprovalSuperseded ApprovalStatus = "superseded"
)

type WorkItemWorkflow struct {
	ID         string `json:"id" yaml:"id"`
	Version    string `json:"version" yaml:"version"`
	EntryPhase string `json:"entry_phase" yaml:"entry_phase"`
}

type WorkItemInput struct {
	Source           string                     `json:"source" yaml:"source"` // user_prompt, external_artifact, imported_artifact
	Summary          string                     `json:"summary" yaml:"summary"`
	References       []string                   `json:"references,omitempty" yaml:"references,omitempty"`
	ExternalArtifact *ExternalArtifactReference `json:"external_artifact,omitempty" yaml:"external_artifact,omitempty"`
}

type ExternalArtifactReference struct {
	Artifact string `json:"artifact" yaml:"artifact"`
	Path     string `json:"path" yaml:"path"`
	SHA256   string `json:"sha256" yaml:"sha256"`
}

type PhaseState struct {
	Status   PhaseStatus `json:"status" yaml:"status"`
	Artifact string      `json:"artifact,omitempty" yaml:"artifact,omitempty"`
}

type Approval struct {
	Phase   string         `json:"phase" yaml:"phase"`
	Status  ApprovalStatus `json:"status" yaml:"status"`
	By      *Actor         `json:"by,omitempty" yaml:"by,omitempty"`
	At      string         `json:"at,omitempty" yaml:"at,omitempty"`
	Comment string         `json:"comment,omitempty" yaml:"comment,omitempty"`
}

type Traceability struct {
	Events           string   `json:"events" yaml:"events"`
	RelatedWorkItems []string `json:"related_work_items,omitempty" yaml:"related_work_items,omitempty"`
	BaselineSpecs    []string `json:"baseline_specs,omitempty" yaml:"baseline_specs,omitempty"`
}

type TokenUsage struct {
	Status           string  `json:"status" yaml:"status"` // not_reported, partial, recorded
	Source           *string `json:"source,omitempty" yaml:"source,omitempty"`
	InputTokens      *int    `json:"input_tokens,omitempty" yaml:"input_tokens,omitempty"`
	OutputTokens     *int    `json:"output_tokens,omitempty" yaml:"output_tokens,omitempty"`
	CacheReadTokens  *int    `json:"cache_read_tokens,omitempty" yaml:"cache_read_tokens,omitempty"`
	CacheWriteTokens *int    `json:"cache_write_tokens,omitempty" yaml:"cache_write_tokens,omitempty"`
}

type Observability struct {
	TokenUsage *TokenUsage `json:"token_usage,omitempty" yaml:"token_usage,omitempty"`
}

type WorkItem struct {
	SchemaVersion string                `json:"schema_version" yaml:"schema_version"`
	Kind          string                `json:"kind" yaml:"kind"`
	ID            string                `json:"id" yaml:"id"`
	Revision      int                   `json:"revision" yaml:"revision"`
	Title         string                `json:"title" yaml:"title"`
	Type          string                `json:"type" yaml:"type"`
	Status        WorkItemStatus        `json:"status" yaml:"status"`
	CreatedAt     string                `json:"created_at" yaml:"created_at"`
	CreatedBy     *Actor                `json:"created_by,omitempty" yaml:"created_by,omitempty"`
	Workflow      WorkItemWorkflow      `json:"workflow" yaml:"workflow"`
	Input         WorkItemInput         `json:"input" yaml:"input"`
	Phases        map[string]PhaseState `json:"phases" yaml:"phases"`
	Approvals     []Approval            `json:"approvals,omitempty" yaml:"approvals,omitempty"`
	Traceability  Traceability          `json:"traceability" yaml:"traceability"`
	Observability *Observability        `json:"observability,omitempty" yaml:"observability,omitempty"`
}

type PhaseTransition struct {
	Phase string
	From  PhaseStatus
	To    PhaseStatus
}

type PhaseMutation struct {
	Transition PhaseTransition
	Unblocked  []PhaseTransition
}

type NextPhase struct {
	Definition WorkflowPhase
	State      PhaseState
}

type NewWorkItemParams struct {
	ID               string
	Title            string
	Summary          string
	EntryPhase       string
	CreatedAt        string
	CreatedBy        Actor
	ExternalArtifact *ExternalArtifactReference
}

func NewWorkItem(workflow *Workflow, params NewWorkItemParams) (*WorkItem, PhaseMutation, error) {
	if workflow == nil {
		return nil, PhaseMutation{}, fmt.Errorf("%w: workflow is required", ErrInvalidWorkItem)
	}
	if err := ValidateIdentifier("work item id", params.ID); err != nil {
		return nil, PhaseMutation{}, err
	}
	if params.Title == "" {
		return nil, PhaseMutation{}, fmt.Errorf("%w: title cannot be empty", ErrInvalidWorkItem)
	}
	if err := ValidateActor(params.CreatedBy); err != nil {
		return nil, PhaseMutation{}, err
	}
	if _, exists := workflow.Phase(params.EntryPhase); !exists {
		return nil, PhaseMutation{}, ErrPhaseNotFound
	}

	phases := make(map[string]PhaseState, len(workflow.Phases))
	for _, phase := range workflow.Phases {
		phases[phase.ID] = PhaseState{
			Status:   PhaseBlocked,
			Artifact: workflow.ArtifactPathForPhase(phase.ID),
		}
	}

	inputSource := "user_prompt"
	if params.ExternalArtifact == nil {
		entryState := phases[params.EntryPhase]
		entryState.Status = PhaseReady
		phases[params.EntryPhase] = entryState
	} else {
		inputSource = "external_artifact"
		ancestors, err := workflow.Ancestors(params.EntryPhase)
		if err != nil {
			return nil, PhaseMutation{}, err
		}
		for phaseID := range ancestors {
			state := phases[phaseID]
			state.Status = PhaseNotApplicable
			phases[phaseID] = state
		}
	}

	item := &WorkItem{
		SchemaVersion: "0.1",
		Kind:          "work-item",
		ID:            params.ID,
		Title:         params.Title,
		Type:          workflow.WorkItemType,
		Status:        WorkItemActive,
		CreatedAt:     params.CreatedAt,
		CreatedBy:     &params.CreatedBy,
		Workflow: WorkItemWorkflow{
			ID:         workflow.ID,
			Version:    workflow.SchemaVersion,
			EntryPhase: params.EntryPhase,
		},
		Input: WorkItemInput{
			Source:           inputSource,
			Summary:          params.Summary,
			ExternalArtifact: params.ExternalArtifact,
		},
		Phases: phases,
		Traceability: Traceability{
			Events: "events.jsonl",
		},
	}

	var (
		mutation PhaseMutation
		err      error
	)
	if params.ExternalArtifact == nil {
		mutation, err = item.BeginPhase(workflow, params.EntryPhase)
	} else {
		mutation, err = item.AcceptExternalPhase(workflow, params.EntryPhase)
	}
	if err != nil {
		return nil, PhaseMutation{}, err
	}

	return item, mutation, nil
}

func (item *WorkItem) BeginPhase(workflow *Workflow, phaseID string) (PhaseMutation, error) {
	phase, state, err := item.phase(workflow, phaseID)
	if err != nil {
		return PhaseMutation{}, err
	}
	if !item.canMutatePhase(phase) {
		return PhaseMutation{}, fmt.Errorf("%w: work item status is %s", ErrInvalidTransition, item.Status)
	}
	if state.Status != PhaseReady && state.Status != PhaseRejected && state.Status != PhaseSuperseded {
		return PhaseMutation{}, invalidPhaseTransition(phaseID, state.Status, PhaseInProgress)
	}

	return item.transition(phaseID, PhaseInProgress), nil
}

func (item *WorkItem) AcceptExternalPhase(workflow *Workflow, phaseID string) (PhaseMutation, error) {
	phase, state, err := item.phase(workflow, phaseID)
	if err != nil {
		return PhaseMutation{}, err
	}
	if item.Status != WorkItemActive {
		return PhaseMutation{}, fmt.Errorf("%w: work item status is %s", ErrInvalidTransition, item.Status)
	}
	if state.Status != PhaseBlocked && state.Status != PhaseReady {
		return PhaseMutation{}, invalidPhaseTransition(phaseID, state.Status, PhaseAccepted)
	}

	target := PhaseAccepted
	if phase.Approval == ApprovalRequired {
		target = PhaseAwaitingApproval
	}

	mutation := item.transition(phaseID, target)
	if target == PhaseAwaitingApproval {
		item.Approvals = append(item.Approvals, Approval{
			Phase:  phaseID,
			Status: ApprovalPending,
		})
	} else {
		mutation.Unblocked = item.unlockReadyPhases(workflow)
	}

	return mutation, nil
}

func (item *WorkItem) DeliverPhase(workflow *Workflow, phaseID string, requestOptionalApproval bool) (PhaseMutation, error) {
	phase, state, err := item.phase(workflow, phaseID)
	if err != nil {
		return PhaseMutation{}, err
	}
	if !item.canMutatePhase(phase) {
		return PhaseMutation{}, fmt.Errorf("%w: work item status is %s", ErrInvalidTransition, item.Status)
	}
	if state.Status != PhaseInProgress {
		return PhaseMutation{}, invalidPhaseTransition(phaseID, state.Status, PhaseCompleted)
	}

	var target PhaseStatus
	switch phase.Approval {
	case ApprovalRequired:
		target = PhaseAwaitingApproval
	case ApprovalOptional:
		if requestOptionalApproval {
			target = PhaseAwaitingApproval
		} else {
			target = PhaseCompleted
		}
	case ApprovalNone:
		if requestOptionalApproval {
			return PhaseMutation{}, fmt.Errorf("%w: phase %s has approval policy %s", ErrApprovalNotAllowed, phaseID, phase.Approval)
		}
		target = PhaseCompleted
	default:
		return PhaseMutation{}, fmt.Errorf("%w: unknown approval policy %q", ErrInvalidTransition, phase.Approval)
	}

	mutation := item.transition(phaseID, target)
	if target == PhaseAwaitingApproval {
		item.Approvals = append(item.Approvals, Approval{
			Phase:  phaseID,
			Status: ApprovalPending,
		})
	} else {
		mutation.Unblocked = item.unlockReadyPhases(workflow)
	}

	return mutation, nil
}

func (item *WorkItem) ApprovePhase(workflow *Workflow, phaseID string, actor Actor, at, comment string) (PhaseMutation, error) {
	phase, state, err := item.phase(workflow, phaseID)
	if err != nil {
		return PhaseMutation{}, err
	}
	if !item.canMutatePhase(phase) {
		return PhaseMutation{}, fmt.Errorf("%w: work item status is %s", ErrInvalidTransition, item.Status)
	}
	if err := ValidateActor(actor); err != nil {
		return PhaseMutation{}, err
	}
	if actor.Kind != ActorHuman {
		return PhaseMutation{}, ErrHumanActorRequired
	}
	if phase.Approval == ApprovalNone {
		return PhaseMutation{}, fmt.Errorf("%w: phase %s has approval policy %s", ErrApprovalNotAllowed, phaseID, phase.Approval)
	}
	if state.Status != PhaseAwaitingApproval {
		return PhaseMutation{}, fmt.Errorf("%w: current status is %s", ErrPhaseNotAwaitingApproval, state.Status)
	}

	mutation := item.transition(phaseID, PhaseApproved)
	item.resolvePendingApproval(phaseID, ApprovalApproved, actor, at, comment)
	mutation.Unblocked = item.unlockReadyPhases(workflow)

	return mutation, nil
}

func (item *WorkItem) RejectPhase(workflow *Workflow, phaseID string, actor Actor, at, comment string) (PhaseMutation, error) {
	phase, state, err := item.phase(workflow, phaseID)
	if err != nil {
		return PhaseMutation{}, err
	}
	if !item.canMutatePhase(phase) {
		return PhaseMutation{}, fmt.Errorf("%w: work item status is %s", ErrInvalidTransition, item.Status)
	}
	if err := ValidateActor(actor); err != nil {
		return PhaseMutation{}, err
	}
	if actor.Kind != ActorHuman {
		return PhaseMutation{}, ErrHumanActorRequired
	}
	if phase.Approval == ApprovalNone {
		return PhaseMutation{}, fmt.Errorf("%w: phase %s has approval policy %s", ErrApprovalNotAllowed, phaseID, phase.Approval)
	}
	if state.Status != PhaseAwaitingApproval {
		return PhaseMutation{}, fmt.Errorf("%w: current status is %s", ErrPhaseNotAwaitingApproval, state.Status)
	}

	mutation := item.transition(phaseID, PhaseRejected)
	item.resolvePendingApproval(phaseID, ApprovalRejected, actor, at, comment)

	return mutation, nil
}

func (item *WorkItem) CompletePhase(workflow *Workflow, phaseID string) (PhaseMutation, error) {
	phase, state, err := item.phase(workflow, phaseID)
	if err != nil {
		return PhaseMutation{}, err
	}
	if !item.canMutatePhase(phase) {
		return PhaseMutation{}, fmt.Errorf("%w: work item status is %s", ErrInvalidTransition, item.Status)
	}
	if state.Status != PhaseApproved && state.Status != PhaseAccepted {
		return PhaseMutation{}, invalidPhaseTransition(phaseID, state.Status, PhaseCompleted)
	}

	mutation := item.transition(phaseID, PhaseCompleted)
	mutation.Unblocked = item.unlockReadyPhases(workflow)

	return mutation, nil
}

func (item *WorkItem) Complete(workflow *Workflow) error {
	if item.Status != WorkItemActive {
		return fmt.Errorf("%w: current status is %s", ErrWorkItemCannotComplete, item.Status)
	}

	for _, phase := range workflow.Phases {
		state, exists := item.Phases[phase.ID]
		if !exists {
			return fmt.Errorf("%w: phase %s has no state", ErrWorkItemCannotComplete, phase.ID)
		}

		if phase.Optional {
			if state.Status == PhaseBlocked || state.Status == PhaseReady || state.Status == PhaseNotApplicable {
				continue
			}
			if !state.Status.satisfiesCompletion() {
				return fmt.Errorf("%w: optional phase %s was started and is %s", ErrWorkItemCannotComplete, phase.ID, state.Status)
			}
			continue
		}

		if !state.Status.satisfiesCompletion() {
			return fmt.Errorf("%w: required phase %s is %s", ErrWorkItemCannotComplete, phase.ID, state.Status)
		}
	}

	item.Status = WorkItemCompleted
	return nil
}

func (item *WorkItem) Archive(workflow *Workflow) error {
	if err := item.archiveEligibility(workflow); err != nil {
		return err
	}
	item.Status = WorkItemArchived
	return nil
}

func (item *WorkItem) archiveEligibility(workflow *Workflow) error {
	if workflow == nil {
		return fmt.Errorf("%w: workflow is required", ErrWorkItemCannotArchive)
	}
	if item.Status != WorkItemCompleted {
		return fmt.Errorf("%w: current status is %s", ErrWorkItemCannotArchive, item.Status)
	}
	for _, phase := range workflow.Phases {
		state, exists := item.Phases[phase.ID]
		if !exists {
			return fmt.Errorf("%w: phase %s has no state", ErrWorkItemCannotArchive, phase.ID)
		}
		if phase.Optional && phase.ID != "archive" &&
			(state.Status == PhaseBlocked || state.Status == PhaseReady || state.Status == PhaseNotApplicable) {
			continue
		}
		if phase.ID != "archive" && !state.Status.satisfiesCompletion() {
			return fmt.Errorf("%w: phase %s is %s", ErrWorkItemCannotArchive, phase.ID, state.Status)
		}
	}
	_, exists := workflow.Phase("archive")
	if !exists {
		return nil
	}
	state, exists := item.Phases["archive"]
	if !exists {
		return fmt.Errorf("%w: phase archive has no state", ErrWorkItemCannotArchive)
	}
	if !state.Status.satisfiesCompletion() {
		return fmt.Errorf("%w: phase archive is %s", ErrWorkItemCannotArchive, state.Status)
	}
	return nil
}

func (item *WorkItem) NextPhase(workflow *Workflow) (*NextPhase, error) {
	orderedPhases, err := workflow.OrderedPhases()
	if err != nil {
		return nil, err
	}
	priorities := []PhaseStatus{PhaseAwaitingApproval, PhaseInProgress, PhaseReady}
	for _, status := range priorities {
		for _, phase := range orderedPhases {
			state, exists := item.Phases[phase.ID]
			if exists && state.Status == status {
				return &NextPhase{Definition: phase, State: state}, nil
			}
		}
	}

	return nil, nil
}

func (item *WorkItem) phase(workflow *Workflow, phaseID string) (WorkflowPhase, PhaseState, error) {
	phase, exists := workflow.Phase(phaseID)
	if !exists {
		return WorkflowPhase{}, PhaseState{}, ErrPhaseNotFound
	}
	state, exists := item.Phases[phaseID]
	if !exists {
		return WorkflowPhase{}, PhaseState{}, ErrPhaseNotFound
	}

	return phase, state, nil
}

func (item *WorkItem) canMutatePhase(phase WorkflowPhase) bool {
	return item.Status == WorkItemActive || (item.Status == WorkItemCompleted && phase.Optional)
}

func (item *WorkItem) transition(phaseID string, target PhaseStatus) PhaseMutation {
	state := item.Phases[phaseID]
	transition := PhaseTransition{Phase: phaseID, From: state.Status, To: target}
	state.Status = target
	item.Phases[phaseID] = state
	return PhaseMutation{Transition: transition}
}

func (item *WorkItem) unlockReadyPhases(workflow *Workflow) []PhaseTransition {
	var transitions []PhaseTransition
	for _, phase := range workflow.Phases {
		state, exists := item.Phases[phase.ID]
		if !exists || state.Status != PhaseBlocked || !item.dependenciesSatisfied(phase) {
			continue
		}

		state.Status = PhaseReady
		if state.Artifact == "" {
			state.Artifact = workflow.ArtifactPathForPhase(phase.ID)
		}
		item.Phases[phase.ID] = state
		transitions = append(transitions, PhaseTransition{
			Phase: phase.ID,
			From:  PhaseBlocked,
			To:    PhaseReady,
		})
	}

	return transitions
}

func (item *WorkItem) dependenciesSatisfied(phase WorkflowPhase) bool {
	for _, requiredPhaseID := range phase.Requires {
		requiredState, exists := item.Phases[requiredPhaseID]
		if !exists || !requiredState.Status.satisfiesDependency() {
			return false
		}
	}

	return true
}

func (item *WorkItem) resolvePendingApproval(phaseID string, status ApprovalStatus, actor Actor, at, comment string) {
	for i := len(item.Approvals) - 1; i >= 0; i-- {
		if item.Approvals[i].Phase == phaseID && item.Approvals[i].Status == ApprovalPending {
			item.Approvals[i].Status = status
			item.Approvals[i].By = &actor
			item.Approvals[i].At = at
			item.Approvals[i].Comment = comment
			return
		}
	}

	item.Approvals = append(item.Approvals, Approval{
		Phase:   phaseID,
		Status:  status,
		By:      &actor,
		At:      at,
		Comment: comment,
	})
}

func (status PhaseStatus) satisfiesDependency() bool {
	return status == PhaseApproved || status == PhaseCompleted || status == PhaseAccepted
}

func (status PhaseStatus) satisfiesCompletion() bool {
	return status.satisfiesDependency() || status == PhaseNotApplicable
}

func invalidPhaseTransition(phaseID string, from, to PhaseStatus) error {
	return fmt.Errorf("%w: phase %s cannot transition from %s to %s", ErrInvalidTransition, phaseID, from, to)
}
