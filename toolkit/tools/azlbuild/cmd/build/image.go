// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package build

import (
	"fmt"

	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
	"github.com/spf13/cobra"
)

type imageBuildOptions struct {
	dailyRepoId string
	dryRun      bool
	imageConfig string
}

var imageOptions imageBuildOptions

var imageCmd = &cobra.Command{
	Use:   "image",
	Short: "Build base image for Azure Linux (does *not* rebuild packages)",
	RunE: func(cc *cobra.Command, args []string) error {
		return buildImage(cmd.CmdEnv)
	},
	SilenceUsage: true,
}

func init() {
	buildCmd.AddCommand(imageCmd)

	imageCmd.Flags().BoolVar(&imageOptions.dryRun, "dry-run", false, "Prepare build environment but do not build")
	imageCmd.Flags().StringVar(&imageOptions.dailyRepoId, "daily-repo", "lkg", "ID of daily repo to use as upstream package cache")

	imageCmd.Flags().StringVarP(&imageOptions.imageConfig, "config", "c", "", "Path to the image config file")
	imageCmd.MarkFlagFilename("config")
}

func buildImage(env *cmd.BuildEnv) error {
	configFilePath, err := env.ResolveImageConfig(imageOptions.imageConfig)
	if err != nil {
		return err
	}

	target := cmd.NewToolkitMakeTarget("image")
	target.RequiresSudo = true
	target.DryRun = imageOptions.dryRun

	extraArgs := []string{
		fmt.Sprintf("CONFIG_FILE=%s", configFilePath),
		"USE_PACKAGE_BUILD_CACHE=y",
		"REBUILD_PACKAGES=n",
		fmt.Sprintf("DAILY_BUILD_ID=%s", packageOptions.dailyRepoId),
	}

	env.RunToolkitMake(target, extraArgs...)

	return nil
}
