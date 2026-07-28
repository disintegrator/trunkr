package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/disintegrator/trunkr/internal/herdr"
)

// helloCommand is the walking-skeleton action: prove we can find wt on PATH
// and call back into herdr. Output lands in the plugin command log.
func helloCommand() *cli.Command {
	return &cli.Command{
		Name:  "hello",
		Usage: "verify trunkr can reach wt and call back into herdr",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			hc, err := herdr.FromEnv()
			if err != nil {
				return err
			}
			wtPath, err := exec.LookPath("wt")
			if err != nil {
				msg := "worktrunk (wt) not found on PATH — install it from https://github.com/max-sixty/worktrunk"
				hc.Notify(ctx, "trunkr: wt missing", msg)
				return fmt.Errorf("%s", msg)
			}

			verOut, err := exec.Command(wtPath, "--version").Output()
			if err != nil {
				return fmt.Errorf("found %s but `wt --version` failed: %w", wtPath, err)
			}
			wtVersion := strings.TrimSpace(string(verOut))

			body := fmt.Sprintf("%s at %s · plugin %s", wtVersion, wtPath, os.Getenv("HERDR_PLUGIN_ID"))
			if err := hc.Notify(ctx, "trunkr says hello", body); err != nil {
				return fmt.Errorf("herdr callback failed: %w", err)
			}

			fmt.Fprintf(cmd.Writer, "hello ok: %s; herdr callback ok (HERDR_BIN_PATH=%s)\n", body, os.Getenv("HERDR_BIN_PATH"))
			return nil
		},
	}
}
