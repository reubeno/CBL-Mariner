// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/artifactcache"
	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/exe"
	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/logger"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	app = kingpin.New("cachectl", "Manages the shared artifact cache")

	logFile  = exe.LogFileFlag(app)
	logLevel = exe.LogLevelFlag(app)

	cacheDir = app.Flag("cache", "Path to artifact cache.").Required().String()

	statsCommand = app.Command("stats", "Prints statistics about the cache.")
)

func main() {
	app.Version(exe.ToolkitVersion)
	selectedCommand := kingpin.MustParse(app.Parse(os.Args[1:]))
	logger.InitBestEffort(*logFile, *logLevel)

	// Open the cache.
	cache, err := artifactcache.Open(*cacheDir)
	if err != nil {
		logger.PanicOnError(err)
	}

	switch selectedCommand {
	case statsCommand.FullCommand():
		err = doStats(cache)
	default:
		err = fmt.Errorf("unknown command: %s", selectedCommand)
	}

	logger.PanicOnError(err)
}

func doStats(cache *artifactcache.ArtifactCache) error {
	var entrySize int64
	entryCount := 0
	err := cache.VisitArtifacts(func(entry *artifactcache.ArtifactCacheEntry, entryErr error) error {
		if entryErr != nil {
			return nil
		}

		entryCount += 1

		thisEntrySize, _ := bestEffortSizeOfDirTree(entry.ContentPath)
		entrySize += thisEntrySize

		return nil
	})

	if err != nil {
		return err
	}

	logger.Log.Infof("Cached artifacts: %d", entryCount)
	logger.Log.Infof("Cache size: %.2f MiB", float64(entrySize)/1024/1024)

	return nil
}

func bestEffortSizeOfDirTree(dirPath string) (size int64, err error) {
	err = filepath.Walk(dirPath, func(path string, info os.FileInfo, walkErr error) error {
		// Keep going on error.
		if walkErr != nil {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		size += info.Size()
		return nil
	})

	return
}
