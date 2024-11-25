// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package download

import (
	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
	"github.com/spf13/cobra"
)

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download Azure Linux components",
}

func init() {
	cmd.RootCmd.AddCommand(downloadCmd)
}
