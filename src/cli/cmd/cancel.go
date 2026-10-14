package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/usecases"
)

// newCancelCommand builds the cobra.Command for `sdd-cli cancel`.
// The command permanently moves an active work item to cancelled/, reserving
// its ID for audit purposes.
func newCancelCommand(useCase *usecases.CancelUseCase, options *rootOptions) *cobra.Command {
	var reason, actorKind, actorID, operationID string
	command := &cobra.Command{
		Use:   "cancel <work-item-id>",
		Short: "Cancel an active work item and reserve its ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			result, err := useCase.Execute(options.targetDir, usecases.CancelInput{
				WorkItemID: args[0],
				Reason:     reason,
				Actor: domain.Actor{
					Kind: domain.ActorKind(actorKind),
					ID:   actorID,
				},
				OperationID: operationID,
			})
			if err != nil {
				return err
			}
			return outputSuccess(command, options.json, result, func(writer io.Writer) error {
				_, err := fmt.Fprintf(writer, "Work item '%s' cancelled.\n", args[0])
				return err
			})
		},
	}
	command.Flags().StringVar(&reason, "reason", "", "Reason for cancellation (mandatory, recorded in audit log)")
	command.Flags().StringVar(&actorKind, "actor-kind", "agent", "Actor kind (human, agent, cli, system)")
	command.Flags().StringVar(&actorID, "actor-id", "agent", "Actor ID")
	command.Flags().StringVar(&operationID, "operation-id", "", "Stable idempotency key for safe retries")
	mustMarkFlagRequired(command, "reason")
	return command
}
