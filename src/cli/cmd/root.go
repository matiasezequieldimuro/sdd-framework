package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"sdd-cli/internal/domain"
)

type rootOptions struct {
	json      bool
	targetDir string
}

type JSONResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *JSONError  `json:"error,omitempty"`
}

type JSONError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

func NewRootCommand(application Application) *cobra.Command {
	options := &rootOptions{}
	root := &cobra.Command{
		Use:           "sdd-cli",
		Short:         "SDD Engine CLI - Spec-Driven Development Framework Engine",
		Long:          "CLI tool for managing Spec-Driven Development workflows, work items, phase state transitions, and event tracking.",
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.SetVersionTemplate("sdd-cli {{.Version}}\n")
	root.PersistentFlags().BoolVar(&options.json, "json", false, "Output results in JSON format")
	root.PersistentFlags().StringVar(&options.targetDir, "dir", ".", "Target project directory path")
	root.AddCommand(
		newInitCommand(application.Init, options),
		newAdaptersCommand(application.Adapters, options),
		newStartCommand(application.Start, options),
		newStatusCommand(application.Status, options),
		newNextCommand(application.Next, options),
		newValidateCommand(application.Validate, options),
		newBeginCommand(application.Begin, options),
		newDeliverCommand(application.Deliver, options),
		newApproveCommand(application.Approve, options),
		newRejectCommand(application.Reject, options),
		newCompleteCommand(application.Complete, options),
		newArchiveCommand(application.Archive, options),
		newRecordEventCommand(application.RecordEvent, options),
		newVersionCommand(options),
	)
	return root
}

func Execute() error {
	root := NewRootCommand(NewProductionApplication())
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)
	return executeRoot(root)
}

func executeRoot(root *cobra.Command) error {
	err := root.Execute()
	if err == nil {
		return nil
	}

	jsonOutput, flagErr := root.PersistentFlags().GetBool("json")
	if flagErr != nil {
		return errors.Join(err, flagErr)
	}
	if outputErr := outputError(root, jsonOutput, err); outputErr != nil {
		return errors.Join(err, outputErr)
	}
	return err
}

func outputSuccess(
	command *cobra.Command,
	jsonOutput bool,
	data interface{},
	textPrinter func(io.Writer) error,
) error {
	if jsonOutput {
		return writeJSON(command.OutOrStdout(), JSONResponse{
			Success: true,
			Data:    data,
		})
	}
	if textPrinter == nil {
		return nil
	}
	return textPrinter(command.OutOrStdout())
}

func outputError(command *cobra.Command, jsonOutput bool, err error) error {
	if jsonOutput {
		var details interface{}
		if detailed, ok := err.(interface{ Details() interface{} }); ok {
			details = detailed.Details()
		}
		return writeJSON(command.OutOrStdout(), JSONResponse{
			Success: false,
			Error: &JSONError{
				Code:    errorCode(err),
				Message: err.Error(),
				Details: details,
			},
		})
	}
	if report, ok := err.(interface{ WriteText(io.Writer) error }); ok {
		return report.WriteText(command.ErrOrStderr())
	}
	_, writeErr := fmt.Fprintf(command.ErrOrStderr(), "Error: %v\n", err)
	return writeErr
}

func writeJSON(writer io.Writer, response JSONResponse) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		return fmt.Errorf("encode JSON response: %w", err)
	}
	return nil
}

func errorCode(err error) string {
	switch {
	case errors.Is(err, domain.ErrWorkItemNotFound),
		errors.Is(err, domain.ErrWorkflowNotFound),
		errors.Is(err, domain.ErrPhaseNotFound),
		errors.Is(err, domain.ErrAdapterNotFound):
		return "not_found"
	case errors.Is(err, domain.ErrWorkItemAlreadyExists):
		return "already_exists"
	case errors.Is(err, domain.ErrAdapterInstallConflict):
		return "already_exists"
	case errors.Is(err, domain.ErrWorkItemAlreadyArchived):
		return "already_archived"
	case errors.Is(err, domain.ErrArchiveConflict):
		return "archive_conflict"
	case errors.Is(err, domain.ErrConcurrentModification):
		return "concurrent_modification"
	case errors.Is(err, domain.ErrWorkItemLocked):
		return "work_item_locked"
	case errors.Is(err, domain.ErrValidationFailed):
		return "validation_failed"
	case errors.Is(err, domain.ErrInvalidTransition),
		errors.Is(err, domain.ErrPhaseNotAwaitingApproval),
		errors.Is(err, domain.ErrPhaseBlocked),
		errors.Is(err, domain.ErrHumanActorRequired),
		errors.Is(err, domain.ErrApprovalNotAllowed),
		errors.Is(err, domain.ErrWorkItemCannotComplete),
		errors.Is(err, domain.ErrWorkItemCannotArchive):
		return "invalid_transition"
	case errors.Is(err, domain.ErrInvalidIdentifier),
		errors.Is(err, domain.ErrInvalidActor),
		errors.Is(err, domain.ErrInvalidPath),
		errors.Is(err, domain.ErrSchemaValidation),
		errors.Is(err, domain.ErrInvalidWorkflow),
		errors.Is(err, domain.ErrInvalidWorkItem),
		errors.Is(err, domain.ErrInvalidEntryPoint),
		errors.Is(err, domain.ErrInvalidExternalArtifact):
		return "invalid_input"
	case strings.Contains(err.Error(), "arg(s)") ||
		strings.Contains(err.Error(), "required flag(s)") ||
		strings.Contains(err.Error(), "unknown command") ||
		strings.Contains(err.Error(), "unknown flag"):
		return "invalid_arguments"
	default:
		return "internal_error"
	}
}

func mustMarkFlagRequired(command *cobra.Command, flag string) {
	if err := command.MarkFlagRequired(flag); err != nil {
		panic(fmt.Sprintf("failed to mark --%s as required: %v", flag, err))
	}
}
