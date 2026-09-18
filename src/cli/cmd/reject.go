package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/usecases"
)

func newRejectCommand(useCase *usecases.RejectUseCase, options *rootOptions) *cobra.Command {
	var phaseID, rejectedBy, comment, operationID string
	command := &cobra.Command{
		Use:   "reject <work-item-id>",
		Short: "Record human rejection for a phase",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			item, err := useCase.Execute(options.targetDir, usecases.RejectInput{
				WorkItemID: args[0],
				PhaseID:    phaseID,
				RejectedBy: domain.Actor{
					Kind: domain.ActorHuman,
					ID:   rejectedBy,
				},
				Comment:     comment,
				OperationID: operationID,
			})
			if err != nil {
				return err
			}
			return outputSuccess(command, options.json, item, func(writer io.Writer) error {
				_, err := fmt.Fprintf(
					writer,
					"Successfully rejected phase '%s' for work item '%s' by %s.\n",
					phaseID,
					args[0],
					rejectedBy,
				)
				return err
			})
		},
	}
	command.Flags().StringVarP(&phaseID, "phase", "p", "", "Phase ID to reject")
	command.Flags().StringVarP(&rejectedBy, "by", "b", "human", "Human reviewer ID")
	command.Flags().StringVarP(&comment, "comment", "c", "", "Optional rejection comment")
	command.Flags().StringVar(&operationID, "operation-id", "", "Stable idempotency key for safe retries")
	mustMarkFlagRequired(command, "phase")
	return command
}
