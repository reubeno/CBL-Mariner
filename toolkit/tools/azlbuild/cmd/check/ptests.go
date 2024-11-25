// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package check

import (
	"log/slog"

	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
	"github.com/spf13/cobra"
)

var ptestsCmd = &cobra.Command{
	Use:   "ptests",
	Short: "Run package tests",
	RunE: func(c *cobra.Command, args []string) error {
		return runPtests(cmd.CmdEnv)
	},
	SilenceUsage: true,
}

func runPtests(env *cmd.BuildEnv) error {
	slog.Info("Running package tests")

	// TODO: implement
	return nil
}

func init() {
	checkCmd.AddCommand(ptestsCmd)
}
