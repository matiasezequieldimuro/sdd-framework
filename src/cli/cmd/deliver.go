package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/usecases"
)

func newDeliverCommand(useCase *usecases.DeliverPhaseUseCase, options *rootOptions) *cobra.Command {
	var (
		phaseID         string
		requestApproval bool
		actorKind       string
		actorID         string
		operationID     string
	)
	command := &cobra.Command{
		Use:   "deliver <work-item-id>",
		Short: "Deliver the output of an in-progress phase",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			item, err := useCase.Execute(options.targetDir, usecases.DeliverPhaseInput{
				WorkItemID:              args[0],
				PhaseID:                 phaseID,
				RequestOptionalApproval: requestApproval,
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
				_, err := fmt.Fprintf(writer, "Phase '%s' delivered for work item '%s'.\n", phaseID, args[0])
				return err
			})
		},
	}
	command.Flags().StringVarP(&phaseID, "phase", "p", "", "Phase ID to deliver")
	command.Flags().BoolVar(&requestApproval, "request-approval", false, "Request human approval for an optional gate")
	command.Flags().StringVar(&actorKind, "actor-kind", "agent", "Actor kind (human, agent, cli, system)")
	command.Flags().StringVar(&actorID, "actor-id", "agent", "Actor ID")
	command.Flags().StringVar(&operationID, "operation-id", "", "Stable idempotency key for safe retries")
	mustMarkFlagRequired(command, "phase")
	return command
}
