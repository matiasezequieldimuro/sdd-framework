package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/usecases"
)

type validationReportError struct{ failure *usecases.ValidationFailure }

func newValidateCommand(useCase *usecases.ValidateUseCase, options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "validate [work-item-id]",
		Short: "Validate the SDD project or one work item",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			workItemID := ""
			if len(args) == 1 {
				workItemID = args[0]
			}
			report, err := useCase.Execute(options.targetDir, workItemID)
			if err != nil {
				return err
			}
			if !report.Valid {
				return &validationReportError{failure: &usecases.ValidationFailure{Report: report}}
			}
			return outputSuccess(command, options.json, report, func(writer io.Writer) error {
				return printValidationReport(writer, report)
			})
		},
	}
}

func (err *validationReportError) Error() string {
	return err.failure.Error()
}

func (err *validationReportError) Unwrap() error {
	return err.failure
}

func (err *validationReportError) Details() interface{} {
	return err.failure.Details()
}

func (err *validationReportError) WriteText(writer io.Writer) error {
	return printValidationReport(writer, err.failure.Report)
}

func printValidationReport(writer io.Writer, report *usecases.ValidationReport) error {
	outcome := "passed"
	if !report.Valid {
		outcome = "failed"
	}
	target := fmt.Sprintf("%q", report.Target)
	if report.Scope == usecases.ValidationScopeWorkItem {
		target = fmt.Sprintf("work item %q", report.Target)
	} else {
		target = fmt.Sprintf("project %q", report.Target)
	}
	if _, err := fmt.Fprintf(writer, "Validation %s for %s.\n", outcome, target); err != nil {
		return err
	}
	for _, check := range report.Checks {
		if check.Status == domain.CheckPassed {
			continue
		}
		if _, err := fmt.Fprintf(
			writer,
			"%s %s %s: %s\n",
			strings.ToUpper(string(check.Status)),
			check.Code,
			check.Target,
			check.Message,
		); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(
		writer,
		"Checks: %d passed, %d warnings, %d failed.\n",
		report.Summary.Passed,
		report.Summary.Warnings,
		report.Summary.Failed,
	)
	return err
}
