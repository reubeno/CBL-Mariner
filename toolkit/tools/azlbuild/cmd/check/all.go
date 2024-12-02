// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package check

import (
	"fmt"

	"github.com/charmbracelet/huh/spinner"
	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
)

type allChecker struct{}

func (allChecker) Name() string {
	return "all"
}

func (allChecker) Description() string {
	return "Runs ALL checks"
}

func (allChecker) CheckSpecs(env *cmd.BuildEnv, checkerCtx *CheckerContext, specPaths []string) []CheckResult {
	var results []CheckResult

	for _, checker := range registeredSpecCheckers {
		if checker.Name() == "all" {
			continue
		}

		var results []CheckResult
		var err error
		spinner.New().Title(fmt.Sprintf("Running check: %s", checker.Name())).Action(func() {
			results, err = runCheckerOnSpecs(checker, &specPaths)
		}).Run()

		if err == nil {
			err = reportCheckerResults(checker, results)
		}

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
