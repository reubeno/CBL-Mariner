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

var toolchainManifestsCmd = &cobra.Command{
	Use:   "toolchain-manifests",
	Short: "Check toolchain manifests",
	RunE: func(c *cobra.Command, args []string) error {
		return checkToolchainManifests(cmd.CmdEnv)
	},
	SilenceUsage: true,
}

func checkToolchainManifests(env *cmd.BuildEnv) error {
	scriptPath := path.Join(env.ToolkitDir, "scripts", "toolchain", "check_manifests.sh")

	for _, arch := range []string{"x86_64", "aarch64"} {
		slog.Info("Checking toolchain manifests", "arch", arch)

		scriptCmd := exec.Command(scriptPath, "-a", arch)
		scriptCmd.Stdout = os.Stdout
		scriptCmd.Stderr = os.Stderr
		scriptCmd.Dir = env.ToolkitDir

		err := scriptCmd.Run()
		if err != nil {
			return err
		}

		slog.Info("Check passed", "arch", arch)
	}

	return nil
}

func init() {
	checkCmd.AddCommand(toolchainManifestsCmd)
}
