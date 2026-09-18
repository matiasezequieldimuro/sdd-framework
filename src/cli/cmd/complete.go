package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/usecases"
)

func newCompleteCommand(useCase *usecases.CompleteUseCase, options *rootOptions) *cobra.Command {
	var phaseID, actorKind, actorID, operationID string
	command := &cobra.Command{
		Use:   "complete <work-item-id>",
		Short: "Complete an approved phase or the work item",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			item, err := useCase.Execute(options.targetDir, usecases.CompleteInput{
				WorkItemID: args[0],
				PhaseID:    phaseID,
				Actor: domain.Actor{
					Kind: domain.ActorKind(actorKind),
					ID:   actorID,
				},
				OperationID: operationID,
			})
			if err != nil {
				return err
			}
			return outputSuccess(command, options.json, item, func(writer io.Writer) error {
				if phaseID == "" {
					_, err := fmt.Fprintf(writer, "Work item '%s' completed.\n", args[0])
					return err
				}
				_, err := fmt.Fprintf(writer, "Phase '%s' completed for work item '%s'.\n", phaseID, args[0])
				return err
			})
		},
	}
	command.Flags().StringVarP(&phaseID, "phase", "p", "", "Approved or accepted phase to complete; omit to complete the work item")
	command.Flags().StringVar(&actorKind, "actor-kind", "cli", "Actor kind (human, agent, cli, system)")
	command.Flags().StringVar(&actorID, "actor-id", "sdd", "Actor ID")
	command.Flags().StringVar(&operationID, "operation-id", "", "Stable idempotency key for safe retries")
	return command
}
