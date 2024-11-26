// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package build

import (
	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build Azure Linux packages and images",
}

func init() {
	cmd.RootCmd.AddCommand(buildCmd)
}
