//go:build mage

// This is a magefile, and is a "makefile for go".
// See https://magefile.org/
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

const releaseVersionBase = "2.0"

var releaseVersion string

func buildExe(name string) error {
	return sh.Run("go", "build",
		fmt.Sprintf("-ldflags=-X github.com/microsoft/CBL-Mariner/toolkit/tools/internal/exe.ToolkitVersion=%s", releaseVersion),
		"-o", fmt.Sprintf("./build/%s", name),
		fmt.Sprintf("./%s/%s.go", name, name))
}

func installExe(name string) error {
	return sh.Run("go", "install", fmt.Sprintf("./%s/%s.go", name, name))
}

func buildBooter() error {
	return buildExe("booter")
}
func buildRoast() error {
	return buildExe("roast")
}
func buildImager() error {
	return buildExe("imager")
}

func installBooter() error {
	return installExe("booter")
}
func installRoast() error {
	return installExe("roast")
}
func installImager() error {
	return installExe("imager")
}

// Builds executable binaries
func Build() {
	now := time.Now()
	releaseDate := now.Format("20060102.1504")
	releaseVersion = fmt.Sprintf("%s.%s", releaseVersionBase, releaseDate)
	mg.Deps(buildRoast, buildImager, buildBooter)
}

// Install executable binaries under GOBIN
func Install() {
	mg.Deps(installRoast, installImager, installBooter)
}

func checkSourceFormatting() error {
	output, err := sh.Output("gofmt", "-l", ".")
	if err != nil {
		return err
	}

	lines := strings.Split(output, "\n")
	filesNeedingFormatting := len(lines)
	if filesNeedingFormatting > 0 && output == "" {
		filesNeedingFormatting -= 1
	}

	if filesNeedingFormatting > 0 {
		return fmt.Errorf("found %d .go files needing reformatting; please run 'go fmt'", filesNeedingFormatting)
	}

	return nil
}

// Checks sources
func Check() {
	mg.Deps(checkSourceFormatting)
}

// Cleans output
func Clean() error {
	return os.RemoveAll("build")
}

// Runs tests
func Test() error {
	return sh.RunV("go", "test", "./...")
}
