// trunkr is a herdr plugin binary. Every manifest entrypoint (actions, panes)
// invokes this binary with a subcommand; herdr sets the plugin root as cwd and
// passes all context via HERDR_* environment variables.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
)

func main() {
	root := &cli.Command{
		Name:  "trunkr",
		Usage: "worktrunk worktrees inside herdr",
		Commands: []*cli.Command{
			helloCommand(),
			helloPaneCommand(),
			actionCommand(),
			runnerCommand(),
		},
	}
	if err := root.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "trunkr: "+err.Error())
		os.Exit(1)
	}
}
