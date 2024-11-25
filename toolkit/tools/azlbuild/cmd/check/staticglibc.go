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

var staticGlibcCmd = &cobra.Command{
	Use:   "static-glibc",
	Short: "Check static glibc",
	RunE: func(c *cobra.Command, args []string) error {
		return checkStaticGlibc(cmd.CmdEnv)
	},
	SilenceUsage: true,
}

func checkStaticGlibc(env *cmd.BuildEnv) error {
	slog.Info("Checking static glibc")

	scriptArgs := []string{
		path.Join(env.ToolkitDir, "scripts", "check_static_glibc.py"),
	}

	matches, err := filepath.Glob(path.Join(env.SpecsDir, "**", "*.spec"))
	if err != nil {
		return err
	}

	scriptArgs = append(scriptArgs, matches...)

	matches, err = filepath.Glob(path.Join(env.ExtendedSpecsDir, "**", "*.spec"))
	if err != nil {
		return err
	}

	scriptArgs = append(scriptArgs, matches...)

	matches, err = filepath.Glob(path.Join(env.SignedSpecsDir, "**", "*.spec"))
	if err != nil {
		return err
	}

	scriptArgs = append(scriptArgs, matches...)

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
	checkCmd.AddCommand(staticGlibcCmd)
}
