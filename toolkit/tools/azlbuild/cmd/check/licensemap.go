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

var licenseMapCmd = &cobra.Command{
	Use:   "license-map",
	Short: "Check license map",
	RunE: func(c *cobra.Command, args []string) error {
		return checkLicenseMap(cmd.CmdEnv)
	},
	SilenceUsage: true,
}

func checkLicenseMap(env *cmd.BuildEnv) error {
	slog.Info("Checking license map")

	scriptArgs := []string{
		path.Join(env.ToolkitDir, "scripts", "license_map.py"),
		path.Join(env.LicensesAndNoticesDir, "SPECS/data/licenses.json"),
		path.Join(env.LicensesAndNoticesDir, "SPECS/LICENSES-MAP.md"),
		env.SpecsDir,
		env.ExtendedSpecsDir,
		env.SignedSpecsDir,
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
	checkCmd.AddCommand(licenseMapCmd)

	// Flags
}
