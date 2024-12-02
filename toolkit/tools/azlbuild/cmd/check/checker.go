package check

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"

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

type CheckerContext struct {
	stdoutLogPath string
	stderrLogPath string
}

func NewCheckerContext(env *cmd.BuildEnv, checker *SpecChecker) (*CheckerContext, error) {
	checkerName := (*checker).Name()

	timeStamp := time.Now().Format("20060102-150405")
	logDirPath := path.Join(env.ChecksLogsDir, "checks", timeStamp)

	err := os.MkdirAll(logDirPath, 0755)
	if err != nil {
		return nil, err
	}

	// Set up an output files for logs.
	stdoutLogPath := path.Join(logDirPath, fmt.Sprintf("%s.stdout.log", checkerName))
	stderrLogPath := path.Join(logDirPath, fmt.Sprintf("%s.stderr.log", checkerName))

	return &CheckerContext{
		stdoutLogPath: stdoutLogPath,
		stderrLogPath: stderrLogPath,
	}, nil
}

type SpecChecker interface {
	Name() string
	Description() string
}

type SingleSpecChecker interface {
	Name() string
	Description() string
	CheckSpec(env *cmd.BuildEnv, checkerCtx *CheckerContext, specPath string) CheckResult
}

type BulkSpecChecker interface {
	Name() string
	Description() string
	CheckSpecs(env *cmd.BuildEnv, checkerCtx *CheckerContext, specPaths []string) []CheckResult
}

type UnscopedSpecChecker interface {
	Name() string
	Description() string
	CheckAllSpecs(env *cmd.BuildEnv, checkerCtx *CheckerContext) []CheckResult
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
		changedSpecPaths, err := cmd.CmdEnv.DetectLikelyChangedFiles(true, true)
		if err != nil {
			return nil
		}

		specPaths = append(specPaths, changedSpecPaths...)
	}

	slog.Debug("Running checker", "checker", checker.Name(), "specs", specPaths)

	return runCheckerOnSpecsAndReport(checker, &specPaths)
}

func runCheckerOnAllSpecs(checker SpecChecker) error {
	if unscopedSpecChecker, valid := checker.(UnscopedSpecChecker); valid {
		checkerCtx, err := NewCheckerContext(cmd.CmdEnv, &checker)
		if err != nil {
			return err
		}

		results := unscopedSpecChecker.CheckAllSpecs(cmd.CmdEnv, checkerCtx)
		return reportCheckerResults(checker, results)
	} else {
		allSpecPaths, err := findAllSpecPaths(cmd.CmdEnv)
		if err != nil {
			return err
		}

		return runCheckerOnSpecsAndReport(checker, &allSpecPaths)
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

func runCheckerOnSpecsAndReport(checker SpecChecker, specPaths *[]string) error {
	results, err := runCheckerOnSpecs(checker, specPaths)
	if err != nil {
		return err
	}

	return reportCheckerResults(checker, results)
}

func runCheckerOnSpecs(checker SpecChecker, specPaths *[]string) ([]CheckResult, error) {
	var results []CheckResult

	checkerCtx, err := NewCheckerContext(cmd.CmdEnv, &checker)
	if err != nil {
		return results, err
	}

	if bulkSpecChecker, valid := checker.(BulkSpecChecker); valid {
		results = bulkSpecChecker.CheckSpecs(cmd.CmdEnv, checkerCtx, *specPaths)
	} else if singleSpecChecker, valid := checker.(SingleSpecChecker); valid {
		for _, specPath := range *specPaths {
			results = append(results, singleSpecChecker.CheckSpec(cmd.CmdEnv, checkerCtx, specPath))
		}
	} else if unscopedSpecChecker, valid := checker.(UnscopedSpecChecker); valid {
		slog.Debug("Running unscoped checker", "checker", checker.Name())
		results = unscopedSpecChecker.CheckAllSpecs(cmd.CmdEnv, checkerCtx)
	} else {
		return results, fmt.Errorf("unsupported checker type: %s", checker.Name())
	}

	return results, nil
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

func RunExternalCheckerCmd(checkerCtx *CheckerContext, cmd *exec.Cmd, specPath string) CheckResult {
	stdoutFile, err := os.Create(checkerCtx.stdoutLogPath)
	if err != nil {
		return CheckResult{
			Status:   CheckInternalError,
			Error:    err,
			SpecPath: specPath,
		}
	}

	defer stdoutFile.Close()

	stderrFile, err := os.Create(checkerCtx.stderrLogPath)
	if err != nil {
		return CheckResult{
			Status:   CheckInternalError,
			Error:    err,
			SpecPath: specPath,
		}
	}

	defer stderrFile.Close()

	// TODO: Write output to file.
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile

	err = cmd.Run()

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
