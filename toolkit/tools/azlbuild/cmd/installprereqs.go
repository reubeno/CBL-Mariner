// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package cmd

import (
	"log/slog"
	"os"
	"os/exec"
	"path"

	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "install-prereqs",
	Short: "Install prerequisites for this tool",
	RunE: func(cmd *cobra.Command, args []string) error {
		return installPrereqs(CmdEnv)
	},
	SilenceUsage: true,
}

func init() {
	RootCmd.AddCommand(checkCmd)
}

func installPrereqs(env *BuildEnv) error {
	slog.Info("Installing prerequisites")

	scriptsDir := path.Join(env.ToolkitDir, "scripts")
	requirementsFilePath := path.Join(scriptsDir, "requirements.txt")

	slog.Debug("Installing script pip requirements", "requirementsFile", requirementsFilePath)

	cmd := exec.Command("pip", "install", "-r", requirementsFilePath, "--user")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
