package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/usecases"
)

// newReworkCommand builds the cobra.Command for `sdd-cli rework`.
// The command resets a completed/approved/accepted phase back to ready and
// supersedes all downstream descendant phases.
func newReworkCommand(useCase *usecases.ReworkUseCase, options *rootOptions) *cobra.Command {
	var phaseID, reason, actorKind, actorID, operationID string
	command := &cobra.Command{
		Use:   "rework <work-item-id>",
		Short: "Reset a phase to ready and supersede its descendants",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			result, err := useCase.Execute(options.targetDir, usecases.ReworkInput{
				WorkItemID: args[0],
				PhaseID:    phaseID,
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
				if result.Reactivated {
					_, err := fmt.Fprintf(
						writer,
						"Work item '%s' reactivated and phase '%s' reworked. %d phase(s) superseded.\n",
						args[0],
						phaseID,
						result.Superseded,
					)
					return err
				}
				_, err := fmt.Fprintf(
					writer,
					"Work item '%s': phase '%s' reworked. %d phase(s) superseded.\n",
					args[0],
					phaseID,
					result.Superseded,
				)
				return err
			})
		},
	}
	command.Flags().StringVarP(&phaseID, "phase", "p", "", "Phase ID to reset to ready")
	command.Flags().StringVar(&reason, "reason", "", "Reason for the rework (mandatory, recorded in audit log)")
	command.Flags().StringVar(&actorKind, "actor-kind", "agent", "Actor kind (human, agent, cli, system)")
	command.Flags().StringVar(&actorID, "actor-id", "agent", "Actor ID")
	command.Flags().StringVar(&operationID, "operation-id", "", "Stable idempotency key for safe retries")
	mustMarkFlagRequired(command, "phase")
	mustMarkFlagRequired(command, "reason")
	return command
}
