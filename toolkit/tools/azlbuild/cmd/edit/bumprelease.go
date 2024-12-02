// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package edit

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
	"github.com/spf13/cobra"
)

var specName string
var changeLogMessage string

var bumpReleaseCmd = &cobra.Command{
	Use:   "bump-release",
	Short: "Bump release on spec",
	RunE: func(c *cobra.Command, args []string) error {
		return bumpRelease(cmd.CmdEnv, specName, changeLogMessage)
	},
	SilenceUsage: true,
}

func bumpRelease(env *cmd.BuildEnv, specName, changeLogMessage string) error {
	matches, err := filepath.Glob(path.Join(env.RepoRootDir, "SPECS*", "**", specName+".spec"))
	if err != nil {
		return err
	}

	if len(matches) == 0 {
		return fmt.Errorf("spec not found: %s", specName)
	}

	if len(matches) > 1 {
		return fmt.Errorf("multiple specs found: %s", matches)
	}

	specPath := matches[0]

	scriptArgs := []string{
		path.Join(env.ToolkitDir, "scripts", "update_spec.sh"),
		changeLogMessage,
		specPath,
	}

	slog.Info("Updating spec", "spec", specPath)

	scriptCmd := exec.Command(scriptArgs[0], scriptArgs[1:]...)
	scriptCmd.Stdout = os.Stdout
	scriptCmd.Stderr = os.Stderr

	return scriptCmd.Run()
}

func init() {
	editCmd.AddCommand(bumpReleaseCmd)

	bumpReleaseCmd.Flags().StringVar(&specName, "spec-name", "", "Name of spec to update")
	bumpReleaseCmd.MarkFlagRequired("spec-name")

	bumpReleaseCmd.Flags().StringVar(&changeLogMessage, "changelog", "Placeholder changelog message.", "Changelog message to use (defaults to placeholder)")
}
