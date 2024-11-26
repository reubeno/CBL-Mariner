// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package query

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
	"github.com/spf13/cobra"
)

var useDailyRepo bool
var useExtendedRepo bool
var dailyRepoId string

var repoqueryCmd = &cobra.Command{
	Use:   "repo",
	Short: "Query published Azure Linux package repos",
	RunE: func(c *cobra.Command, args []string) error {
		return repoquery(cmd.CmdEnv, args)
	},
	SilenceUsage: true,
	Example: `  Query the production RPM repo for packages that provide '/bin/sh':
    azlbuild query repo -- --whatprovides /bin/sh

  Query the last-known-good daily dev repo for the 'dtc' package:
    azlbuild query repo --daily dtc
`,
}

func repoquery(env *cmd.BuildEnv, extraArgs []string) error {
	var err error
	var baseUris []string
	if useDailyRepo {
		if dailyRepoId == "lkg" {
			dailyRepoId, err = env.GetLkgDailyRepoId()
			if err != nil {
				return err
			}
		}

		baseUri, err := env.GetDailyRepoBaseUri(dailyRepoId)
		if err != nil {
			return err
		}

		baseUris = []string{baseUri}
	} else {
		baseUris, err = env.GetProdRepoBaseUris(useExtendedRepo)
		if err != nil {
			return err
		}
	}

	dnfArgs := []string{
		"--quiet",
		"--disablerepo=*",
	}

	for i, uri := range baseUris {
		dnfArgs = append(dnfArgs, fmt.Sprintf("--repofrompath=azl%d,%s", i, uri))
	}

	dnfArgs = append(dnfArgs, "repoquery")
	dnfArgs = append(dnfArgs, extraArgs...)

	dnfCmd := exec.Command("dnf", dnfArgs...)
	dnfCmd.Stdout = os.Stdout
	dnfCmd.Stderr = os.Stderr

	return dnfCmd.Run()
}

func init() {
	queryCmd.AddCommand(repoqueryCmd)

	repoqueryCmd.Flags().BoolVar(&useDailyRepo, "daily", false, "Use daily repo")
	repoqueryCmd.Flags().StringVar(&dailyRepoId, "daily-id", "lkg", "ID of daily repo to use")
}
