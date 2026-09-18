package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/usecases"
)

func newRecordEventCommand(useCase *usecases.RecordEventUseCase, options *rootOptions) *cobra.Command {
	var eventType, message, actorKind, actorID, operationID string
	command := &cobra.Command{
		Use:   "record-event <work-item-id>",
		Short: "Append a custom event to the work item event log",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := useCase.Execute(options.targetDir, usecases.RecordEventInput{
				WorkItemID: args[0],
				EventType:  eventType,
				Message:    message,
				Actor: domain.Actor{
					Kind: domain.ActorKind(actorKind),
					ID:   actorID,
				},
				OperationID: operationID,
			}); err != nil {
				return err
			}
			return outputSuccess(
				command,
				options.json,
				"Event recorded successfully",
				func(writer io.Writer) error {
					_, err := fmt.Fprintf(
						writer,
						"Successfully recorded event '%s' for work item '%s'.\n",
						eventType,
						args[0],
					)
					return err
				},
			)
		},
	}
	command.Flags().StringVarP(&eventType, "type", "t", "", "Event type")
	command.Flags().StringVarP(&message, "message", "m", "", "Event message description")
	command.Flags().StringVar(&actorKind, "actor-kind", "agent", "Actor kind (human, agent, cli, system)")
	command.Flags().StringVar(&actorID, "actor-id", "agent", "Actor ID")
	command.Flags().StringVar(&operationID, "operation-id", "", "Stable idempotency key for safe retries")
	mustMarkFlagRequired(command, "type")
	return command
}
