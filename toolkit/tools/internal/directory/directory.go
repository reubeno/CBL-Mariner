// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package directory

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/reubeno/CBL-Mariner/toolkit/tools/internal/file"
	"github.com/reubeno/CBL-Mariner/toolkit/tools/internal/shell"
)

// LastModifiedFile returns the timestamp and path to the file last modified inside a directory.
// Will recursively search.
func LastModifiedFile(dirPath string) (modTime time.Time, latestFile string, err error) {
	err = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		currentModTime := info.ModTime()
		if currentModTime.After(modTime) {
			modTime = currentModTime
			latestFile = path
		}

		return nil
	})

	return
}

// CopyContents will recursively copy the contents of srcDir into dstDir.
// - It will create dstDir if it does not already exist.
func CopyContents(srcDir, dstDir string) (err error) {
	const squashErrors = false

	isSrcDir, err := file.IsDir(srcDir)
	if err != nil {
		return err
	}

	if !isSrcDir {
		return fmt.Errorf("source (%s) must be a directory", srcDir)
	}

	err = os.MkdirAll(dstDir, os.ModePerm)
	if err != nil {
		return
	}

	fds, err := ioutil.ReadDir(srcDir)
	if err != nil {
		return
	}

	for _, fd := range fds {
		srcPath := filepath.Join(srcDir, fd.Name())
		dstPath := filepath.Join(dstDir, fd.Name())

		cpArgs := []string{"-a", srcPath, dstPath}
		if fd.IsDir() {
			cpArgs = append([]string{"-r"}, cpArgs...)
		}

		err = shell.ExecuteLive(squashErrors, "cp", cpArgs...)
		if err != nil {
			return
		}
	}

	return

}
