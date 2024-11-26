// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package check

import (
	"os/exec"
	"path"

	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
)

type entangledSpecsChecker struct{}

func (entangledSpecsChecker) Name() string {
	return "entangled-specs"
}

func (entangledSpecsChecker) Description() string {
	return "Check entangled specs"
}

func (entangledSpecsChecker) CheckAllSpecs(env *cmd.BuildEnv) []CheckResult {
	scriptArgs := []string{
		path.Join(env.ToolkitDir, "scripts", "check_entangled_specs.py"),
		env.RepoRootDir,
	}

	// TODO: Check Python prerequisites.
	scriptCmd := exec.Command("python3", scriptArgs...)

	result := RunExternalCheckerCmd(scriptCmd, "")
	return []CheckResult{result}
}

func init() {
	registerChecker(entangledSpecsChecker{})
}
