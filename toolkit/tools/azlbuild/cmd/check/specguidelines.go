// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package check

import (
	"os/exec"
	"path"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
)

type guidelineChecker struct{}

func (guidelineChecker) Name() string {
	return "spec-guidelines"
}

func (guidelineChecker) Description() string {
	return "Check spec guidelines"
}

func (c guidelineChecker) CheckSpecs(env *cmd.BuildEnv, specPaths []string) []CheckResult {
	var results []CheckResult

	for _, specPath := range specPaths {
		result := c.CheckSpec(env, specPath)
		results = append(results, result)
	}

	return results
}

func (c guidelineChecker) CheckSpec(env *cmd.BuildEnv, specPath string) CheckResult {
	absSpecPath, err := filepath.Abs(specPath)
	if err != nil {
		return CheckResult{
			SpecPath: specPath,
			Status:   CheckInternalError,
			Error:    err,
		}
	}

	scriptArgs := []string{
		path.Join(env.ToolkitDir, "scripts", "check_spec_guidelines.py"),
		"--specs",
		absSpecPath,
	}

	// TODO: Check Python prerequisites.
	scriptCmd := exec.Command("python3", scriptArgs...)
	scriptCmd.Dir = env.RepoRootDir

	return RunExternalCheckerCmd(scriptCmd, specPath)
}

func init() {
	registerChecker(guidelineChecker{})
}
