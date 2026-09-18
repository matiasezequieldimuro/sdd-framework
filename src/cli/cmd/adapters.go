package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"sdd-cli/internal/ports"
	"sdd-cli/internal/usecases"
)

type adaptersListResult struct {
	Adapters []ports.AdapterDescriptor `json:"adapters"`
}

func newAdaptersCommand(application AdaptersApplication, options *rootOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "adapters",
		Short: "List and install supported coding-agent adapters",
	}
	command.AddCommand(
		newAdaptersListCommand(application.List, options),
		newAdaptersInstallCommand(application.Install, options),
	)
	return command
}

func newAdaptersListCommand(useCase *usecases.ListAdaptersUseCase, options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List adapters supported by this sdd-cli build",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			adapters, err := useCase.Execute()
			if err != nil {
				return err
			}
			result := adaptersListResult{Adapters: adapters}
			return outputSuccess(command, options.json, result, func(writer io.Writer) error {
				if _, err := fmt.Fprintln(writer, "SUPPORTED ADAPTERS"); err != nil {
					return err
				}
				for _, adapter := range adapters {
					if _, err := fmt.Fprintf(writer, "%-20s %s\n", adapter.ID, adapter.Description); err != nil {
						return err
					}
				}
				return nil
			})
		},
	}
}

func newAdaptersInstallCommand(useCase *usecases.InstallAdapterUseCase, options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "install <adapter-id>",
		Short: "Install one supported adapter into an initialized SDD project",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			result, err := useCase.Execute(options.targetDir, args[0])
			if err != nil {
				return err
			}
			return outputSuccess(command, options.json, result, func(writer io.Writer) error {
				_, err := fmt.Fprintf(
					writer,
					"Adapter %q installed with %d file(s).\n",
					result.ID,
					len(result.Files),
				)
				return err
			})
		},
	}
}
