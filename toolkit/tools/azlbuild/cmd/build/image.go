// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package build

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
	"github.com/spf13/cobra"
)

type imageBuildOptions struct {
	dailyRepoId    string
	dryRun         bool
	configFilePath string
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

	imageCmd.Flags().StringVarP(&imageOptions.configFilePath, "config", "c", "", "Path to the image config file")
	imageCmd.MarkFlagFilename("config")
}

func buildImage(env *cmd.BuildEnv) error {
	configFilePath, err := resolveConfigFile(env, imageOptions.configFilePath)
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

func resolveConfigFile(env *cmd.BuildEnv, specifiedConfigFile string) (string, error) {
	// Make sure *something* was specified.
	if specifiedConfigFile == "" {
		slog.Error("config file is required; you may either specify a default configuration or a full path to a .json image config file")

		foundConfigFilePaths, _ := filepath.Glob(path.Join(env.ToolkitDir, "imageconfigs", "*.json"))
		configsToAdvertise := []string{}
		for _, filePath := range foundConfigFilePaths {
			if !strings.HasSuffix(filePath, ".json") {
				continue
			}

			name := strings.TrimSuffix(filepath.Base(filePath), ".json")
			configsToAdvertise = append(configsToAdvertise, name)
		}

		if len(configsToAdvertise) > 0 {
			fmt.Fprintf(os.Stderr, "Available default configurations:\n")
			for _, name := range configsToAdvertise {
				fmt.Fprintf(os.Stderr, "  %s\n", name)
			}
		}

		return "", fmt.Errorf("no config file path provided")
	}

	// See if the file exists.
	_, err := os.Stat(specifiedConfigFile)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}

		// See if it's a relative stem name of a config file under the `imageconfigs` dir?
		candidatePath := path.Join(env.ToolkitDir, "imageconfigs", specifiedConfigFile+".json")
		if _, otherErr := os.Stat(candidatePath); otherErr == nil {
			specifiedConfigFile = candidatePath
		} else {
			return "", err
		}
	}

	absConfigFilePath, err := filepath.Abs(specifiedConfigFile)
	if err != nil {
		return "", err
	}

	return absConfigFilePath, nil
}
