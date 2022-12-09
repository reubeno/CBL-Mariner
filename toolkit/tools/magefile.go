//go:build mage

// This is a magefile, and is a "makefile for go".
// See https://magefile.org/
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

func buildExe(name string) error {
	return sh.Run("go", "build", "-o", fmt.Sprintf("./build/%s", name), fmt.Sprintf("./%s/%s.go", name, name))
}

func installExe(name string) error {
	return sh.Run("go", "install", fmt.Sprintf("./%s/%s.go", name, name))
}

func buildRoast() error {
	return buildExe("roast")
}
func buildImager() error {
	return buildExe("imager")
}
func buildBooter() error {
	return buildExe("booter")
}

func installRoast() error {
	return installExe("roast")
}
func installImager() error {
	return installExe("imager")
}
func installBooter() error {
	return installExe("booter")
}

// Builds executable binaries
func Build() {
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
