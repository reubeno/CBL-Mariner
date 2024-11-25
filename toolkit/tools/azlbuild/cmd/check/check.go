// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package check

import (
	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
	"github.com/spf13/cobra"
)

var checkChangedSpecsOnly bool

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Run checks against Azure Linux artifacts",
}

func init() {
	cmd.RootCmd.AddCommand(checkCmd)

	checkCmd.PersistentFlags().BoolVar(&checkChangedSpecsOnly, "changed-only", false, "Check changed specs only")
}
