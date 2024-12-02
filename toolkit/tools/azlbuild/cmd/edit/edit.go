// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package edit

import (
	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit Azure Linux specs",
}

func init() {
	cmd.RootCmd.AddCommand(editCmd)
}
