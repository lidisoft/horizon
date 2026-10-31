package cli

import "github.com/urfave/cli/v3"

// adminCommand groups instance administration (docs/operations.md#admin-cli).
// Its subcommands arrive with the modules they act on: users and workspaces
// with identity (phase 2), reconcile, rekey and rebuild-reports later.
func adminCommand() *cli.Command {
	return &cli.Command{
		Name:  "admin",
		Usage: "Administer the instance",
	}
}
