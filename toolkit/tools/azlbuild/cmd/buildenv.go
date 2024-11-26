package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

type BuildEnv struct {
	RepoRootDir           string
	ToolkitDir            string
	LicensesAndNoticesDir string
	SpecsDir              string
	ExtendedSpecsDir      string
	SignedSpecsDir        string

	verbose bool
	quiet   bool
}

func NewBuildEnv(toolkitDir, repoRoot string, verbose bool, quiet bool) *BuildEnv {
	return &BuildEnv{
		RepoRootDir:           repoRoot,
		ToolkitDir:            toolkitDir,
		LicensesAndNoticesDir: path.Join(repoRoot, "LICENSES-AND-NOTICES"),
		SpecsDir:              path.Join(repoRoot, "SPECS"),
		ExtendedSpecsDir:      path.Join(repoRoot, "SPECS-EXTENDED"),
		SignedSpecsDir:        path.Join(repoRoot, "SPECS-SIGNED"),

		verbose: verbose,
		quiet:   quiet,
	}
}

func (env *BuildEnv) GetDistTag() (string, error) {
	target := NewToolkitMakeTarget("get-dist-tag")
	target.RunQuietly = true

	output, err := env.RunToolkitMakeAndGetOutput(target)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(output), nil
}

func (env *BuildEnv) GetSpecRootDirs() []string {
	return []string{env.SpecsDir, env.ExtendedSpecsDir, env.SignedSpecsDir}
}

func (env *BuildEnv) FindSpecByName(specName string) (string, error) {
	specRootDirs := env.GetSpecRootDirs()

	// Look in the most obvious places first.
	for _, specRootDir := range specRootDirs {
		candidatePath := path.Join(specRootDir, specName, fmt.Sprintf("%s.spec", specName))
		if _, err := os.Stat(candidatePath); err == nil {
			return candidatePath, nil
		}
	}

	// Fall back to looking more deeply.
	for _, specRootDir := range specRootDirs {
		matches, err := filepath.Glob(path.Join(specRootDir, "*", fmt.Sprintf("%s.spec", specName)))
		if err != nil {
			return "", err
		}

		if len(matches) > 0 {
			return matches[0], nil
		}
	}

	return "", fmt.Errorf("could not find spec: %s", specName)
}

type ToolkitMakeTarget struct {
	MakeTarget   string
	SpecsDir     string
	RebuildTools bool
	RequiresSudo bool
	RunQuietly   bool
}

func NewToolkitMakeTarget(name string) ToolkitMakeTarget {
	return ToolkitMakeTarget{
		MakeTarget:   name,
		RebuildTools: true,
	}
}

func (env *BuildEnv) RunToolkitMakeAndGetOutput(target ToolkitMakeTarget, additionalArgs ...string) (string, error) {
	makeCmd, err := env.ToolkitMakeCmd(target, additionalArgs...)
	if err != nil {
		return "", err
	}

	slog.Debug("Running toolkit make", "target", target.MakeTarget, "makeCmd", makeCmd)

	output, err := makeCmd.Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

func (env *BuildEnv) RunToolkitMake(target ToolkitMakeTarget, additionalArgs ...string) error {
	makeCmd, err := env.ToolkitMakeCmd(target, additionalArgs...)
	if err != nil {
		return err
	}

	makeCmd.Stdout = os.Stdout
	makeCmd.Stderr = os.Stderr

	return makeCmd.Run()
}

func (env *BuildEnv) ToolkitMakeCmd(target ToolkitMakeTarget, additionalArgs ...string) (*exec.Cmd, error) {
	// Compute effective verbosity level.
	quiet := env.quiet || target.RunQuietly
	verbose := env.verbose

	var toolkitLogLevel string
	if quiet {
		toolkitLogLevel = "warn"
	} else if verbose {
		toolkitLogLevel = "debug"
	} else {
		toolkitLogLevel = "info"
	}

	makeArgs := []string{
		"make",
		"-j",
		fmt.Sprintf("%d", runtime.NumCPU()),
		"-C",
		env.ToolkitDir,
	}

	if target.RequiresSudo {
		makeArgs = append([]string{"sudo"}, makeArgs...)
	}

	if quiet || !verbose {
		makeArgs = append(makeArgs, "--quiet", "--no-print-directory")
	}

	makeArgs = append(
		makeArgs,
		target.MakeTarget,
		fmt.Sprintf("REBUILD_TOOLS=%s", BoolToYN(target.RebuildTools)),
		"GOFLAGS=-buildvcs=false",
		fmt.Sprintf("LOG_LEVEL=%s", toolkitLogLevel),
	)

	if target.SpecsDir != "" {
		makeArgs = append(makeArgs, fmt.Sprintf("SPECS_DIR=%s", target.SpecsDir))
	}

	if len(additionalArgs) > 0 {
		makeArgs = append(makeArgs, additionalArgs...)
	}

	makeCmd := exec.Command(makeArgs[0], makeArgs[1:]...)

	return makeCmd, nil
}

func BoolToYN(b bool) string {
	if b {
		return "y"
	}
	return "n"
}

func (env *BuildEnv) DetectLikelyChangedSpecs() ([]string, error) {
	scriptPath := path.Join(env.ToolkitDir, "scripts", "detect_changes.py")

	scriptCmd := exec.Command("python3", scriptPath, "--include-uncommitted")

	output, err := scriptCmd.Output()
	if err != nil {
		return []string{}, err
	}

	specs := []string{}

	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && strings.HasSuffix(line, ".spec") {
			specs = append(specs, line)
		}
	}

	slog.Debug("Detected likely changed specs", "specs", specs)

	return specs, nil
}
