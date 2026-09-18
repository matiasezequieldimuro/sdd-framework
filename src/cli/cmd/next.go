package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"sdd-cli/internal/usecases"
)

func newNextCommand(useCase *usecases.NextUseCase, options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "next <work-item-id>",
		Short: "Get the next active phase and required procedure",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			action, err := useCase.Execute(options.targetDir, args[0])
			if err != nil {
				return err
			}
			return outputSuccess(command, options.json, action, func(writer io.Writer) error {
				if _, err := fmt.Fprintln(writer, action.Message); err != nil {
					return err
				}
				if action.PhaseID == "" {
					return nil
				}
				if _, err := fmt.Fprintf(writer, "Phase: %s (Status: %s)\n", action.PhaseID, action.Status); err != nil {
					return err
				}
				if _, err := fmt.Fprintf(writer, "Procedure: %s\n", action.Procedure); err != nil {
					return err
				}
				_, err := fmt.Fprintf(writer, "Artifact: %s\n", action.Artifact)
				return err
			})
		},
	}
}
