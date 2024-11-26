// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package query

import (
	"fmt"

	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
	"github.com/spf13/cobra"
)

var excludeUncommittedChanges bool
var onlyShowSpecs bool

var queryChangedCmd = &cobra.Command{
	Use:   "changes",
	Short: "Query changes in working tree",
	RunE: func(c *cobra.Command, args []string) error {
		return queryChanged(cmd.CmdEnv)
	},
	SilenceUsage: true,
}

func queryChanged(env *cmd.BuildEnv) error {
	specs, err := env.DetectLikelyChangedFiles(!excludeUncommittedChanges, onlyShowSpecs)
	if err != nil {
		return err
	}

	for _, spec := range specs {
		fmt.Printf("%s\n", spec)
	}

	return nil
}

func init() {
	queryCmd.AddCommand(queryChangedCmd)

	queryChangedCmd.Flags().BoolVar(&excludeUncommittedChanges, "exclude-uncommitted", false, "Exclude uncommitted changes")
	queryChangedCmd.Flags().BoolVar(&onlyShowSpecs, "specs-only", false, "Only show changed spec files")
}
