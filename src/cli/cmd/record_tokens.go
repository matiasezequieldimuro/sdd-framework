package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/usecases"
)

// newRecordTokensCommand builds the "record-tokens" sub-command. The command
// accepts the four session-cumulative counters and a source identifier, then
// delegates persistence to the RecordTokensUseCase. The total_tokens field
// is computed by the engine (D-a); it is not a command flag.
func newRecordTokensCommand(useCase *usecases.RecordTokensUseCase, options *rootOptions) *cobra.Command {
	var (
		inputTokens      int
		outputTokens     int
		cacheReadTokens  int
		cacheWriteTokens int
		source           string
		actorKind        string
		actorID          string
		operationID      string
	)

	command := &cobra.Command{
		Use:   "record-tokens <work-item-id>",
		Short: "Record cumulative token usage for an active work item",
		Long: `Persist the cumulative token consumption of the current session into the
work item manifest. The engine overwrites any previous value with the
supplied counters and computes total_tokens internally (D-a).

All four counter flags are required and must be >= 0. Use --source to
identify the adapter or agent performing the recording. Idempotent retries
are supported via --operation-id.`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := useCase.Execute(options.targetDir, usecases.RecordTokensInput{
				WorkItemID:       args[0],
				InputTokens:      inputTokens,
				OutputTokens:     outputTokens,
				CacheReadTokens:  cacheReadTokens,
				CacheWriteTokens: cacheWriteTokens,
				Source:           source,
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
				"Token usage recorded successfully",
				func(writer io.Writer) error {
					_, err := fmt.Fprintf(
						writer,
						"Token usage recorded for work item '%s' (source: %s).\n",
						args[0],
						source,
					)
					return err
				},
			)
		},
	}

	command.Flags().IntVar(&inputTokens, "input-tokens", 0, "Cumulative input token count for the session")
	command.Flags().IntVar(&outputTokens, "output-tokens", 0, "Cumulative output token count for the session")
	command.Flags().IntVar(&cacheReadTokens, "cache-read-tokens", 0, "Cumulative cache-read token count for the session")
	command.Flags().IntVar(&cacheWriteTokens, "cache-write-tokens", 0, "Cumulative cache-write token count for the session")
	command.Flags().StringVar(&source, "source", "", "Identifier of the agent or adapter performing the recording")
	command.Flags().StringVar(&actorKind, "actor-kind", "agent", "Actor kind (human, agent, cli, system)")
	command.Flags().StringVar(&actorID, "actor-id", "agent", "Actor ID")
	command.Flags().StringVar(&operationID, "operation-id", "", "Stable idempotency key for safe retries")

	mustMarkFlagRequired(command, "input-tokens")
	mustMarkFlagRequired(command, "output-tokens")
	mustMarkFlagRequired(command, "cache-read-tokens")
	mustMarkFlagRequired(command, "cache-write-tokens")
	mustMarkFlagRequired(command, "source")

	return command
}
