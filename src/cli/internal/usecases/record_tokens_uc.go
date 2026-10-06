package usecases

import (
	"fmt"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/ports"
)

// RecordTokensInput carries the parameters required to record a token usage
// snapshot for a work item. All four counter fields must be >= 0. The engine
// computes total_tokens = in+out+cacheRead+cacheWrite (D-a).
type RecordTokensInput struct {
	// WorkItemID identifies the target work item.
	WorkItemID string
	// InputTokens is the cumulative input token count for the full session.
	InputTokens int
	// OutputTokens is the cumulative output token count for the full session.
	OutputTokens int
	// CacheReadTokens is the cumulative cache-read token count for the full session.
	CacheReadTokens int
	// CacheWriteTokens is the cumulative cache-write token count for the full session.
	CacheWriteTokens int
	// Source identifies the agent (adapter) performing the recording. Must be non-empty.
	Source string
	// Actor is the agent performing the operation.
	Actor domain.Actor
	// OperationID is an optional idempotency key (RC-9). A non-empty value that
	// was already applied is silently ignored without modifying state.
	OperationID string
}

// RecordTokensUseCase persists a token usage snapshot to a work item manifest.
// It mirrors RecordEventUseCase in structure (minimal orchestrator, delegates
// business rules to the domain model). The use case depends on the mutation
// port, a clock, and an ID generator; it has no knowledge of LLM internals
// (RC-3).
type RecordTokensUseCase struct {
	workItemRepo ports.WorkItemMutationRepository
	clock        ports.Clock
	idGenerator  ports.IDGenerator
}

// NewRecordTokensUseCase constructs a RecordTokensUseCase with the required
// dependencies injected via their port interfaces (DIP).
func NewRecordTokensUseCase(
	repo ports.WorkItemMutationRepository,
	clock ports.Clock,
	idGenerator ports.IDGenerator,
) *RecordTokensUseCase {
	return &RecordTokensUseCase{
		workItemRepo: repo,
		clock:        clock,
		idGenerator:  idGenerator,
	}
}

// Execute records token usage for the work item identified by in.WorkItemID.
//
// Flow: load item → idempotency check → RecordTokenUsage (domain) →
// emit tokens.recorded event → commitWorkItem (transactional write).
//
// Returns domain.ErrTokenAuditInactive if the work item was initialised
// without an active audit. Returns domain.ErrValidationFailed for invalid
// counter values or an empty source. Other errors indicate infrastructure
// failures.
func (uc *RecordTokensUseCase) Execute(baseDir string, in RecordTokensInput) error {
	if err := domain.ValidateActor(in.Actor); err != nil {
		return err
	}
	item, err := uc.workItemRepo.GetWorkItem(baseDir, in.WorkItemID)
	if err != nil {
		return err
	}

	applied, err := operationApplied(baseDir, in.WorkItemID, in.OperationID, uc.workItemRepo)
	if err != nil {
		return err
	}
	if applied {
		return nil
	}

	if err := item.RecordTokenUsage(
		in.InputTokens,
		in.OutputTokens,
		in.CacheReadTokens,
		in.CacheWriteTokens,
		in.Source,
	); err != nil {
		return err
	}

	// Read the total computed by the domain (D-a / F4): avoids duplicating the
	// in+out+cacheRead+cacheWrite arithmetic here and guarantees that the event
	// log always matches the value persisted in the manifest.
	total := *item.Observability.TokenUsage.TotalTokens
	event, err := newOperationEvent(
		in.WorkItemID,
		"tokens.recorded",
		in.Actor,
		map[string]interface{}{
			"input_tokens":      in.InputTokens,
			"output_tokens":     in.OutputTokens,
			"cache_read_tokens": in.CacheReadTokens,
			"cache_write_tokens": in.CacheWriteTokens,
			"total_tokens":      total,
			"source":            in.Source,
		},
		in.OperationID,
		uc.clock,
		uc.idGenerator,
	)
	if err != nil {
		return fmt.Errorf("failed to generate tokens.recorded event: %w", err)
	}

	if _, err := commitWorkItem(baseDir, uc.workItemRepo, item, nil, []domain.Event{event}, in.OperationID); err != nil {
		return fmt.Errorf("failed to commit token usage: %w", err)
	}

	return nil
}
