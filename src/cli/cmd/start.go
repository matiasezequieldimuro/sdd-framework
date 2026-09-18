package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/usecases"
)

func newStartCommand(useCase *usecases.StartWorkItemUseCase, options *rootOptions) *cobra.Command {
	var (
		workflowID   string
		title        string
		summary      string
		fromArtifact string
		phaseID      string
		actorKind    string
		actorID      string
		operationID  string
	)

	command := &cobra.Command{
		Use:   "start <work-item-id>",
		Short: "Start a new SDD work item",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			item, err := useCase.Execute(options.targetDir, usecases.StartWorkItemInput{
				ID:           args[0],
				WorkflowID:   workflowID,
				Title:        title,
				Summary:      summary,
				FromArtifact: fromArtifact,
				Phase:        phaseID,
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
				_, err := fmt.Fprintf(
					writer,
					"Work item '%s' successfully started with workflow '%s'.\n",
					item.ID,
					item.Workflow.ID,
				)
				return err
			})
		},
	}
	command.Flags().StringVarP(&workflowID, "workflow", "w", "", "Workflow ID; defaults to .sdd/config.yaml")
	command.Flags().StringVarP(&title, "title", "t", "", "Work item title")
	command.Flags().StringVarP(&summary, "summary", "s", "", "Initial input summary")
	command.Flags().StringVar(&fromArtifact, "from-artifact", "", "Path to pre-existing artifact")
	command.Flags().StringVar(&phaseID, "phase", "", "Entry phase when using --from-artifact")
	command.Flags().StringVar(&actorKind, "actor-kind", "human", "Actor kind (human, agent, cli)")
	command.Flags().StringVar(&actorID, "actor-id", "user", "Actor ID")
	command.Flags().StringVar(&operationID, "operation-id", "", "Stable idempotency key for safe retries")
	mustMarkFlagRequired(command, "title")
	return command
}
