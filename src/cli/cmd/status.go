package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/usecases"
)

// newStatusCommand builds the "status" sub-command. The text output includes
// phase details and, when present, the token usage block (A-7 / E-10).
// The JSON output already exposes the full work item including Observability.
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
				if err := printTokenUsage(writer, result.Observability); err != nil {
					return err
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

// printTokenUsage writes the Token Usage section to writer when the work item
// has an active or inactive audit block. Nothing is written when Observability
// or TokenUsage is nil (e.g. pre-feature items, E-11).
func printTokenUsage(writer io.Writer, obs *domain.Observability) error {
	if obs == nil || obs.TokenUsage == nil {
		return nil
	}
	tu := obs.TokenUsage
	if _, err := fmt.Fprintln(writer, "-------------------------------------------------------------"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "Token Usage: [%s]\n", tu.Status); err != nil {
		return err
	}
	source := "<none>"
	if tu.Source != nil {
		source = *tu.Source
	}
	if _, err := fmt.Fprintf(writer, "  Source:       %s\n", source); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "  Input:        %s\n", formatOptionalInt(tu.InputTokens)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "  Output:       %s\n", formatOptionalInt(tu.OutputTokens)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "  Cache read:   %s\n", formatOptionalInt(tu.CacheReadTokens)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "  Cache write:  %s\n", formatOptionalInt(tu.CacheWriteTokens)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "  Total:        %s\n", formatOptionalInt(tu.TotalTokens)); err != nil {
		return err
	}
	return nil
}

// formatOptionalInt returns the decimal representation of the pointer value,
// or "<null>" when the pointer is nil (inactive audit counters).
func formatOptionalInt(v *int) string {
	if v == nil {
		return "<null>"
	}
	return fmt.Sprintf("%d", *v)
}
