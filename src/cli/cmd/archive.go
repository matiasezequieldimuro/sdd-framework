package cmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/usecases"
)

func newArchiveCommand(useCase *usecases.ArchiveUseCase, options *rootOptions) *cobra.Command {
	var actorKind, actorID, operationID string
	command := &cobra.Command{
		Use:   "archive <work-item-id>",
		Short: "Archive a completed work item",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			result, err := useCase.Execute(options.targetDir, usecases.ArchiveInput{
				WorkItemID: args[0],
				Actor: domain.Actor{
					Kind: domain.ActorKind(actorKind),
					ID:   actorID,
				},
				OperationID: operationID,
			})
			if err != nil {
				var failure *usecases.ValidationFailure
				if errors.As(err, &failure) {
					return &validationReportError{failure: failure}
				}
				return err
			}
			return outputSuccess(command, options.json, result, func(writer io.Writer) error {
				_, err := fmt.Fprintf(
					writer,
					"Work item '%s' archived at '%s'.\n",
					args[0],
					result.ArchivePath,
				)
				return err
			})
		},
	}
	command.Flags().StringVar(&actorKind, "actor-kind", "cli", "Actor kind (human, agent, cli, system)")
	command.Flags().StringVar(&actorID, "actor-id", "sdd", "Actor ID")
	command.Flags().StringVar(&operationID, "operation-id", "", "Stable idempotency key for safe retries")
	return command
}
