// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package build

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
	"github.com/spf13/cobra"
)

type packageBuildOptions struct {
	dailyRepoId       string
	forceRebuild      bool
	runCheck          bool
	buildChangedSpecs bool
	dryRun            bool
}

var packageOptions packageBuildOptions

var packageCmd = &cobra.Command{
	Use:   "package",
	Short: "Build specific packages for Azure Linux",
	RunE: func(cc *cobra.Command, args []string) error {
		return buildPackages(cmd.CmdEnv, args)
	},
	SilenceUsage: true,
}

func init() {
	buildCmd.AddCommand(packageCmd)

	packageCmd.Flags().BoolVar(&packageOptions.dryRun, "dry-run", false, "Prepare build environment but do not build")
	packageCmd.Flags().BoolVarP(&packageOptions.runCheck, "check", "c", false, "Run package %check tests")
	packageCmd.Flags().BoolVar(&packageOptions.buildChangedSpecs, "changed", false, "Build specs that appear to have been changed")
	packageCmd.Flags().StringVar(&packageOptions.dailyRepoId, "daily-repo", "lkg", "ID of daily repo to use as upstream package cache")
	packageCmd.Flags().BoolVarP(&packageOptions.forceRebuild, "force-rebuild", "f", false, "Force rebuild of specs")
}

func buildPackages(env *cmd.BuildEnv, specNames []string) error {
	if packageOptions.buildChangedSpecs {
		specPaths, err := env.DetectLikelyChangedFiles(true, true)
		if err != nil {
			return err
		}

		// TODO: Handle SPECS-EXTENDED
		for _, specPath := range specPaths {
			filename := filepath.Base(specPath)
			specName := strings.TrimSuffix(filename, ".spec")
			specNames = append(specNames, specName)
		}
	}

	if len(specNames) == 0 {
		return fmt.Errorf("no specs found to build")
	}

	slog.Info("Building packages from specs", "specs", specNames)

	target := cmd.NewToolkitMakeTarget("build-packages")
	target.RequiresSudo = true
	target.DryRun = packageOptions.dryRun

	specNameList := strings.Join(specNames, " ")

	extraArgs := []string{
		"QUICK_REBUILD_PACKAGES=y",
		"USE_PACKAGE_BUILD_CACHE=y",
		"USE_CCACHE=y",
		fmt.Sprintf("RUN_CHECK=%s", cmd.BoolToYN(packageOptions.runCheck)),
		fmt.Sprintf("DAILY_BUILD_ID=%s", packageOptions.dailyRepoId),
		fmt.Sprintf("SRPM_PACK_LIST=%s", specNameList),
	}

	if packageOptions.forceRebuild {
		extraArgs = append(extraArgs, fmt.Sprintf("PACKAGE_REBUILD_LIST=%s", specNameList))
	}

	env.RunToolkitMake(target, extraArgs...)

	return nil
}
