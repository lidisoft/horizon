// Package cli builds the urfave/cli command tree.
//
// Configuration comes from HORIZON_* environment variables only
// (docs/operations.md#configuration); flags are reserved for behaviour that
// differs between invocations, such as serve --with-worker.
package cli

import (
	"github.com/urfave/cli/v3"

	"github.com/lidisoft/horizon/internal/version"
)

// New returns the root command.
func New() *cli.Command {
	return &cli.Command{
		Name:    "horizon",
		Usage:   "Project management: issues, planning and reports",
		Version: version.String(),
		Commands: []*cli.Command{
			serveCommand(),
			workerCommand(),
			migrateCommand(),
			adminCommand(),
		},
	}
}
