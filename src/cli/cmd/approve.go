package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/usecases"
)

func newApproveCommand(useCase *usecases.ApproveUseCase, options *rootOptions) *cobra.Command {
	var phaseID, approvedBy, comment, operationID string
	command := &cobra.Command{
		Use:   "approve <work-item-id>",
		Short: "Record human approval for a phase",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			item, err := useCase.Execute(options.targetDir, usecases.ApproveInput{
				WorkItemID: args[0],
				PhaseID:    phaseID,
				ApprovedBy: domain.Actor{
					Kind: domain.ActorHuman,
					ID:   approvedBy,
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
					"Successfully approved phase '%s' for work item '%s' by %s.\n",
					phaseID,
					args[0],
					approvedBy,
				)
				return err
			})
		},
	}
	command.Flags().StringVarP(&phaseID, "phase", "p", "", "Phase ID to approve")
	command.Flags().StringVarP(&approvedBy, "by", "b", "human", "Human approver ID")
	command.Flags().StringVarP(&comment, "comment", "c", "", "Optional approval comment")
	command.Flags().StringVar(&operationID, "operation-id", "", "Stable idempotency key for safe retries")
	mustMarkFlagRequired(command, "phase")
	return command
}
