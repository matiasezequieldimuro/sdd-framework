package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"sdd-cli/internal/usecases"
)

func newInitCommand(useCase *usecases.InitUseCase, options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize .sdd framework structure in the project",
		RunE: func(command *cobra.Command, args []string) error {
			if err := useCase.Execute(options.targetDir); err != nil {
				return err
			}
			return outputSuccess(
				command,
				options.json,
				"Successfully initialized .sdd directory framework",
				func(writer io.Writer) error {
					_, err := fmt.Fprintf(
						writer,
						"Successfully initialized .sdd directory framework in '%s'\n",
						options.targetDir,
					)
					return err
				},
			)
		},
	}
}
