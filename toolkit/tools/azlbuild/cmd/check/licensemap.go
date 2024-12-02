// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package check

import (
	"os/exec"
	"path"

	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
)

type licenseMapChecker struct{}

func (licenseMapChecker) Name() string {
	return "license-map"
}

func (licenseMapChecker) Description() string {
	return "Check license map"
}

func (licenseMapChecker) CheckAllSpecs(env *cmd.BuildEnv, checkerCtx *CheckerContext) []CheckResult {
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

	result := RunExternalCheckerCmd(checkerCtx, scriptCmd, "")
	return []CheckResult{result}
}

func init() {
	registerChecker(licenseMapChecker{})
}
