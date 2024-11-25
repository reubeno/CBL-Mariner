// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package check

import (
	"log/slog"
	"os"
	"os/exec"
	"path"

	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
	"github.com/spf13/cobra"
)

var entangledSpecsCmd = &cobra.Command{
	Use:   "entangled-specs",
	Short: "Check entangled specs",
	RunE: func(c *cobra.Command, args []string) error {
		return checkEntangledSpecs(cmd.CmdEnv)
	},
	SilenceUsage: true,
}

func checkEntangledSpecs(env *cmd.BuildEnv) error {
	slog.Info("Checking entangled specs")

	scriptArgs := []string{
		path.Join(env.ToolkitDir, "scripts", "check_entangled_specs.py"),
		env.RepoRootDir,
	}

	// TODO: Check Python prerequisites.
	scriptCmd := exec.Command("python3", scriptArgs...)
	scriptCmd.Stdout = os.Stdout
	scriptCmd.Stderr = os.Stderr

	err := scriptCmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func init() {
	checkCmd.AddCommand(entangledSpecsCmd)
}
