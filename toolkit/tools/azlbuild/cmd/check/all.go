// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package check

import (
	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
)

type allChecker struct{}

func (allChecker) Name() string {
	return "all"
}

func (allChecker) Description() string {
	return "Runs ALL checks"
}

func (allChecker) CheckSpecs(env *cmd.BuildEnv, specPaths []string) []CheckResult {
	var results []CheckResult

	for _, checker := range registeredSpecCheckers {
		if checker.Name() == "all" {
			continue
		}

		err := runCheckerOnSpecs(checker, &specPaths)
		if err != nil {
			results = append(results, CheckResult{
				Status: CheckInternalError,
				Error:  err,
			})
		}
	}

	return results
}

func init() {
	registerChecker(allChecker{})
}
