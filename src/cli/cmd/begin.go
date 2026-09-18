package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/usecases"
)

func newBeginCommand(useCase *usecases.BeginPhaseUseCase, options *rootOptions) *cobra.Command {
	var phaseID, actorKind, actorID, operationID string
	command := &cobra.Command{
		Use:   "begin <work-item-id>",
		Short: "Begin a ready, rejected, or superseded phase",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			item, err := useCase.Execute(options.targetDir, usecases.BeginPhaseInput{
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
				_, err := fmt.Fprintf(writer, "Phase '%s' started for work item '%s'.\n", phaseID, args[0])
				return err
			})
		},
	}
	command.Flags().StringVarP(&phaseID, "phase", "p", "", "Phase ID to begin")
	command.Flags().StringVar(&actorKind, "actor-kind", "agent", "Actor kind (human, agent, cli, system)")
	command.Flags().StringVar(&actorID, "actor-id", "agent", "Actor ID")
	command.Flags().StringVar(&operationID, "operation-id", "", "Stable idempotency key for safe retries")
	mustMarkFlagRequired(command, "phase")
	return command
}
