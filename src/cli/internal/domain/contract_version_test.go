package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"sdd-cli/internal/domain"
)

func TestValidateContractSchemaVersion(t *testing.T) {
	if err := domain.ValidateContractSchemaVersion("subject", domain.ContractSchemaVersion); err != nil {
		t.Fatalf("ValidateContractSchemaVersion(current) error = %v", err)
	}
	for _, version := range []string{"0.1", "", "1.0.0"} {
		err := domain.ValidateContractSchemaVersion("config.yaml", version)
		if err == nil {
			t.Fatalf("ValidateContractSchemaVersion(%q) succeeded", version)
		}
		if !errors.Is(err, domain.ErrSchemaValidation) {
			t.Fatalf("error = %v, want ErrSchemaValidation", err)
		}
		for _, want := range []string{"config.yaml", `"` + version + `"`, `"1.0"`} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("error %q does not contain %q", err.Error(), want)
			}
		}
	}
}

func TestNewEventUsesContractSchemaVersion(t *testing.T) {
	event := domain.NewEvent("evt", time.Now(), "item", "work_item.created",
		domain.Actor{Kind: domain.ActorCLI, ID: "sdd"}, nil)
	if event.SchemaVersion != domain.ContractSchemaVersion {
		t.Fatalf("event schema_version = %q, want %q", event.SchemaVersion, domain.ContractSchemaVersion)
	}
}

func TestNewWorkItemUsesContractSchemaVersion(t *testing.T) {
	workflow := reworkWorkflow()
	item, _, err := domain.NewWorkItem(workflow, domain.NewWorkItemParams{
		ID:         "contract-item",
		Title:      "Contract item",
		EntryPhase: "plan",
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
		CreatedBy:  domain.Actor{Kind: domain.ActorCLI, ID: "sdd"},
	})
	if err != nil {
		t.Fatalf("NewWorkItem() error = %v", err)
	}
	if item.SchemaVersion != domain.ContractSchemaVersion || item.Workflow.Version != domain.ContractSchemaVersion {
		t.Fatalf("versions = %q / %q, want %q", item.SchemaVersion, item.Workflow.Version, domain.ContractSchemaVersion)
	}
}
