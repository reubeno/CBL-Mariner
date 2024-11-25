// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package check

import (
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
	"github.com/spf13/cobra"
)

var specPath string

var specGuidelinesCmd = &cobra.Command{
	Use:   "spec-guidelines",
	Short: "Check spec guidelines",
	RunE: func(c *cobra.Command, args []string) error {
		return checkSpecGuidelines(specPath, cmd.CmdEnv)
	},
	SilenceUsage: true,
}

func checkSpecGuidelines(specPath string, env *cmd.BuildEnv) error {
	slog.Info("Checking spec guidelines", "spec", specPath)

	absSpecPath, err := filepath.Abs(specPath)
	if err != nil {
		return err
	}

	scriptArgs := []string{
		path.Join(env.ToolkitDir, "scripts", "check_spec_guidelines.py"),
		"--specs",
		absSpecPath,
	}

	// TODO: Check Python prerequisites.
	scriptCmd := exec.Command("python3", scriptArgs...)
	scriptCmd.Stdout = os.Stdout
	scriptCmd.Stderr = os.Stderr
	scriptCmd.Dir = env.RepoRootDir

	err = scriptCmd.Run()
	if err != nil {
		return err
	}

	slog.Info("Check passed")

	return nil
}

func init() {
	checkCmd.AddCommand(specGuidelinesCmd)

	specGuidelinesCmd.Flags().StringVarP(&specPath, "spec", "s", "", "spec file path")
	specGuidelinesCmd.MarkFlagRequired("spec")
	specGuidelinesCmd.MarkFlagFilename("spec")
}
