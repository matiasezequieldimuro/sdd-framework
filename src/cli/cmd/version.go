package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// Build metadata. These values are overridden at build time via -ldflags, e.g.
//
//	go build -ldflags "-X sdd-cli/cmd.version=v0.1.0 -X sdd-cli/cmd.commit=abc1234 -X sdd-cli/cmd.date=2026-09-16"
//
// When built without ldflags (local development) they keep the defaults below.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type versionInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

func newVersionCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print sdd-cli version, commit and build date",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			info := versionInfo{Version: version, Commit: commit, Date: date}
			return outputSuccess(command, options.json, info, func(writer io.Writer) error {
				_, err := fmt.Fprintf(writer, "sdd-cli %s (commit %s, built %s)\n", info.Version, info.Commit, info.Date)
				return err
			})
		},
	}
}
