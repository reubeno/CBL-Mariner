// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/lmittmann/tint"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var (
	explicitToolkitDir string
	verbose            bool
	quiet              bool

	CmdEnv  *BuildEnv
	RootCmd = &cobra.Command{
		Use:   "azlbuild",
		Short: "Azure Linux Build Tool",
		Long:  `Build tool for Azure Linux`,
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := RootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Define flags and configuration settings.
	RootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	RootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "only enable minimal output")
	RootCmd.PersistentFlags().StringVarP(&explicitToolkitDir, "toolkit", "C", "", "path to Azure Linux toolkit")
}

func initConfig() {
	initLogging()

	toolkitDir, err := resolveToolkitDir()
	if err != nil {
		cobra.CheckErr(err)
	}

	repoRootDir, err := resolveRepoRootDir()
	if err != nil {
		cobra.CheckErr(err)
	}

	CmdEnv = NewBuildEnv(toolkitDir, repoRootDir, verbose, quiet)
}

func resolveToolkitDir() (string, error) {
	if explicitToolkitDir != "" {
		// Make sure that there's a Makefile there.
		_, err := os.Stat(path.Join(explicitToolkitDir, "Makefile"))
		if err != nil {
			return "", err
		}

		return explicitToolkitDir, nil
	} else {
		// Start at the current directory, and keep going up until we find what looks
		// to be the root of the Azure Linux git repo.
		currentPath, err := filepath.Abs(".")
		if err != nil {
			return "", err
		}

		for {
			candidatePath := path.Join(currentPath, "toolkit")

			_, err := os.Stat(candidatePath)
			if err == nil {
				return candidatePath, nil
			}

			if currentPath == "/" {
				return "", fmt.Errorf("could not find Azure Linux toolkit")
			}

			currentPath = path.Dir(currentPath)
		}
	}
}

func resolveRepoRootDir() (string, error) {
	// Start at the current directory, and keep going up until we find what looks
	// to be the root of the Azure Linux git repo.
	currentPath, err := filepath.Abs(".")
	if err != nil {
		return "", err
	}

	for {
		candidatePath := path.Join(currentPath, ".git")

		_, err := os.Stat(candidatePath)
		if err == nil {
			return currentPath, nil
		}

		if currentPath == "/" {
			return "", fmt.Errorf("could not find Azure Linux repo root")
		}

		currentPath = path.Dir(currentPath)
	}
}

func initLogging() {
	w := os.Stderr

	var logLevel slog.Level
	if verbose {
		logLevel = slog.LevelDebug
	} else if quiet {
		logLevel = slog.LevelWarn
	} else {
		logLevel = slog.LevelInfo
	}

	// set global logger with custom options
	slog.SetDefault(slog.New(
		tint.NewHandler(w, &tint.Options{
			Level:      logLevel,
			TimeFormat: time.Kitchen,
		}),
	))

	// Also initialize the logrus logger
	logger.InitStderrLog()
}
