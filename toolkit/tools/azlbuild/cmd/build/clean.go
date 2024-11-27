// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package build

import (
	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
	"github.com/spf13/cobra"
)

var cleanDryRun bool

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean build cache",
	RunE: func(cc *cobra.Command, args []string) error {
		return cleanBuildCache(cmd.CmdEnv)
	},
	SilenceUsage: true,
}

func init() {
	buildCmd.AddCommand(cleanCmd)

	cleanCmd.Flags().BoolVar(&cleanDryRun, "dry-run", false, "Dry run only (don't actually clean anything)")
}

func cleanBuildCache(env *cmd.BuildEnv) error {
	target := cmd.NewToolkitMakeTarget("clean")
	target.RequiresSudo = true
	target.DryRun = cleanDryRun

	env.RunToolkitMake(target)

	return nil
}
