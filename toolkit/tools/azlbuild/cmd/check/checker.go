package check

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
	"github.com/spf13/cobra"
)

type CheckStatus int

const (
	CheckSucceeded     CheckStatus = iota
	CheckFailed        CheckStatus = iota
	CheckSkipped       CheckStatus = iota
	CheckInternalError CheckStatus = iota
)

type CheckResult struct {
	// Required
	Status CheckStatus

	// Optional
	SpecPath string
	Error    error
}

type SpecChecker interface {
	Name() string
	Description() string
}

type SingleSpecChecker interface {
	Name() string
	Description() string
	CheckSpec(env *cmd.BuildEnv, specPath string) CheckResult
}

type BulkSpecChecker interface {
	Name() string
	Description() string
	CheckSpecs(env *cmd.BuildEnv, specPaths []string) []CheckResult
}

type UnscopedSpecChecker interface {
	Name() string
	Description() string
	CheckAllSpecs(env *cmd.BuildEnv) []CheckResult
}

var registeredSpecCheckers []SpecChecker

type specCheckerOptions struct {
	// Receives paths to specs to process.
	specPaths []string
	// Receives simple names of specs to process (not full paths)
	specNames []string
	// If set, we should try to test *all* specs.
	allSpecs bool
	// If set, we try to infer which specs have changed and test *them*.
	changedSpecs bool
}

func registerChecker(checker SpecChecker) {
	registeredSpecCheckers = append(registeredSpecCheckers, checker)

	var options specCheckerOptions
	checkerCmd := &cobra.Command{
		Use:   checker.Name(),
		Short: checker.Description(),
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("invalid usage")
			}

			return runChecker(checker, &options)
		},
		SilenceUsage: true,
	}

	checkCmd.AddCommand(checkerCmd)

	checkerCmd.Flags().StringArrayVarP(&options.specPaths, "spec-path", "p", []string{}, "Path to spec file to check")
	checkerCmd.MarkFlagFilename("spec")

	checkerCmd.Flags().StringArrayVarP(&options.specNames, "spec-name", "n", []string{}, "Name of spec file to check")

	checkerCmd.Flags().BoolVarP(&options.allSpecs, "all", "a", false, "Check all specs")
	checkerCmd.Flags().BoolVarP(&options.changedSpecs, "changed", "c", false, "Check *changed* specs")
}

func runChecker(checker SpecChecker, options *specCheckerOptions) error {
	if options.allSpecs {
		return runCheckerOnAllSpecs(checker)
	}

	specPaths := options.specPaths

	for _, specName := range options.specNames {
		specPath, err := cmd.CmdEnv.FindSpecByName(specName)
		if err != nil {
			return err
		}

		specPaths = append(specPaths, specPath)
	}

	if options.changedSpecs {
		changedSpecPaths, err := cmd.CmdEnv.DetectLikelyChangedSpecs()
		if err != nil {
			return nil
		}

		specPaths = append(specPaths, changedSpecPaths...)
	}

	slog.Debug("Running checker", "checker", checker.Name(), "specs", specPaths)

	return runCheckerOnSpecs(checker, &specPaths)
}

func runCheckerOnAllSpecs(checker SpecChecker) error {
	if unscopedSpecChecker, valid := checker.(UnscopedSpecChecker); valid {
		results := unscopedSpecChecker.CheckAllSpecs(cmd.CmdEnv)
		return reportCheckerResults(checker, results)
	} else {
		allSpecPaths, err := findAllSpecPaths(cmd.CmdEnv)
		if err != nil {
			return err
		}

		return runCheckerOnSpecs(checker, &allSpecPaths)
	}
}

func findAllSpecPaths(env *cmd.BuildEnv) ([]string, error) {
	var allMatches []string

	matches, err := filepath.Glob(path.Join(env.SpecsDir, "**", "*.spec"))
	if err != nil {
		return []string{}, err
	}

	allMatches = append(allMatches, matches...)

	matches, err = filepath.Glob(path.Join(env.ExtendedSpecsDir, "**", "*.spec"))
	if err != nil {
		return []string{}, err
	}

	allMatches = append(allMatches, matches...)

	matches, err = filepath.Glob(path.Join(env.SignedSpecsDir, "**", "*.spec"))
	if err != nil {
		return []string{}, err
	}

	allMatches = append(allMatches, matches...)

	return allMatches, nil
}

func runCheckerOnSpecs(checker SpecChecker, specPaths *[]string) error {
	var results []CheckResult
	if bulkSpecChecker, valid := checker.(BulkSpecChecker); valid {
		results = bulkSpecChecker.CheckSpecs(cmd.CmdEnv, *specPaths)
	} else if singleSpecChecker, valid := checker.(SingleSpecChecker); valid {
		for _, specPath := range *specPaths {
			results = append(results, singleSpecChecker.CheckSpec(cmd.CmdEnv, specPath))
		}
	} else if unscopedSpecChecker, valid := checker.(UnscopedSpecChecker); valid {
		slog.Debug("Running unscoped checker", "checker", checker.Name())
		results = unscopedSpecChecker.CheckAllSpecs(cmd.CmdEnv)
	} else {
		return fmt.Errorf("unsupported checker type: %s", checker.Name())
	}

	return reportCheckerResults(checker, results)
}

func reportCheckerResults(checker SpecChecker, results []CheckResult) error {
	color.Set(color.Underline, color.Italic)
	fmt.Fprintf(os.Stderr, "Check: %s\n", checker.Name())
	color.Unset()

	var err error
	for _, result := range results {
		returnError := false

		specPath := result.SpecPath

		var specToDisplay string
		if specPath != "" {
			specToDisplay = filepath.Base(specPath)
		} else {
			specToDisplay = "(all)"
		}

		switch result.Status {
		case CheckSucceeded:
			fmt.Fprintf(os.Stderr, "✅ PASS: %s\n", specToDisplay)
		case CheckFailed:
			fmt.Fprintf(os.Stderr, "❌ FAIL: %s\n", specToDisplay)
			returnError = true
		case CheckSkipped:
			fmt.Fprintf(os.Stderr, "⏩ SKIPPED: %s\n", specToDisplay)
		case CheckInternalError:
			fmt.Fprintf(os.Stderr, "⛔ INTERNAL ERROR: %s (%v)\n", specToDisplay, result.Error)
			returnError = true
		}

		if returnError && err == nil {
			err = fmt.Errorf("one or more checks failed")
		}
	}

	fmt.Fprintf(os.Stderr, "\n")

	return err
}

func RunExternalCheckerCmd(cmd *exec.Cmd, specPath string) CheckResult {
	// TODO: Write output to file.
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr

	err := cmd.Run()

	// Check if the error was because of a non-zero exit.
	var status CheckStatus
	if _, isExitError := err.(*exec.ExitError); isExitError {
		status = CheckFailed
	} else if err != nil {
		status = CheckInternalError
	} else {
		status = CheckSucceeded
	}

	return CheckResult{
		Status:   status,
		Error:    err,
		SpecPath: specPath,
	}
}
