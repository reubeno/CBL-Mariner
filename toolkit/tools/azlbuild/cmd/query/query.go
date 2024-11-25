// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package query

import (
	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
	"github.com/spf13/cobra"
)

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Query Azure Linux components",
}

func init() {
	cmd.RootCmd.AddCommand(queryCmd)
}
