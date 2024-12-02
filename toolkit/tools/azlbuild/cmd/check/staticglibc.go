// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package check

import (
	"os/exec"
	"path"

	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
)

type staticGlibcChecker struct{}

func (staticGlibcChecker) Name() string {
	return "static-glibc"
}

func (staticGlibcChecker) Description() string {
	return "Check static glibc"
}

func (c staticGlibcChecker) CheckSpecs(env *cmd.BuildEnv, checkerCtx *CheckerContext, specPaths []string) []CheckResult {
	scriptArgs := []string{
		path.Join(env.ToolkitDir, "scripts", "check_static_glibc.py"),
	}

	scriptArgs = append(scriptArgs, specPaths...)

	// TODO: Check Python prerequisites.
	scriptCmd := exec.Command("python3", scriptArgs...)
	scriptCmd.Dir = env.RepoRootDir

	result := RunExternalCheckerCmd(checkerCtx, scriptCmd, "")

	var results []CheckResult
	for _, specPath := range specPaths {
		result.SpecPath = specPath
		results = append(results, result)
	}

	return results
}

func init() {
	registerChecker(staticGlibcChecker{})
}
