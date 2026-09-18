package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"sdd-cli/internal/usecases"
)

func newStatusCommand(useCase *usecases.StatusUseCase, options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "status <work-item-id>",
		Short: "Get work item status and phase details",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			result, err := useCase.Execute(options.targetDir, args[0])
			if err != nil {
				return err
			}
			return outputSuccess(command, options.json, result, func(writer io.Writer) error {
				if _, err := fmt.Fprintf(writer, "Work Item: %s [%s]\n", result.ID, result.Status); err != nil {
					return err
				}
				if _, err := fmt.Fprintf(writer, "Title: %s\nWorkflow: %s\n", result.Title, result.Workflow.ID); err != nil {
					return err
				}
				if _, err := fmt.Fprintf(writer, "Location: %s\n", result.Location); err != nil {
					return err
				}
				if result.ArchivePath != "" {
					if _, err := fmt.Fprintf(writer, "Archive path: %s\n", result.ArchivePath); err != nil {
						return err
					}
				}
				if _, err := fmt.Fprintln(writer, "-------------------------------------------------------------"); err != nil {
					return err
				}
				if _, err := fmt.Fprintf(writer, "%-20s %-20s %-20s\n", "PHASE", "STATUS", "ARTIFACT"); err != nil {
					return err
				}
				if _, err := fmt.Fprintln(writer, "-------------------------------------------------------------"); err != nil {
					return err
				}
				for _, phase := range result.OrderedPhases {
					if _, err := fmt.Fprintf(writer, "%-20s %-20s %-20s\n", phase.ID, phase.Status, phase.Artifact); err != nil {
						return err
					}
				}
				return nil
			})
		},
	}
}
