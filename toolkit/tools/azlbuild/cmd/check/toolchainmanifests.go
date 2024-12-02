// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package check

import (
	"fmt"
	"os/exec"
	"path"

	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
)

type toolchainManifestChecker struct{}

func (toolchainManifestChecker) Name() string {
	return "toolchain-manifests"
}

func (toolchainManifestChecker) Description() string {
	return "Check toolchain manifests"
}

func (toolchainManifestChecker) CheckAllSpecs(env *cmd.BuildEnv, checkerCtx *CheckerContext) []CheckResult {
	scriptPath := path.Join(env.ToolkitDir, "scripts", "toolchain", "check_manifests.sh")

	results := []CheckResult{}
	for _, arch := range []string{"x86_64", "aarch64"} {
		scriptCmd := exec.Command(scriptPath, "-a", arch)
		scriptCmd.Dir = env.ToolkitDir

		result := RunExternalCheckerCmd(checkerCtx, scriptCmd, fmt.Sprintf("(%s)", arch))
		results = append(results, result)
	}

	return results
}

func init() {
	registerChecker(toolchainManifestChecker{})
}
