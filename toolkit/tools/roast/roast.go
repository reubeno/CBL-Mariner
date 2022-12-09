// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// A raw to other format converter

package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"github.com/reubeno/CBL-Mariner/toolkit/tools/imagegen/configuration"
	"github.com/reubeno/CBL-Mariner/toolkit/tools/internal/exe"
	"github.com/reubeno/CBL-Mariner/toolkit/tools/internal/file"
	"github.com/reubeno/CBL-Mariner/toolkit/tools/internal/logger"
	"github.com/reubeno/CBL-Mariner/toolkit/tools/roast/formats"

	"gopkg.in/alecthomas/kingpin.v2"
)

const defaultWorkerCount = "10"

type convertRequest struct {
	inputPath   string
	isInputFile bool
	artifact    configuration.Artifact
}

type convertResult struct {
	artifactName  string
	originalPath  string
	convertedFile string
}

var (
	app = kingpin.New("roast", "A tool to convert raw disk file into another image type")

	logFile  = exe.LogFileFlag(app)
	logLevel = exe.LogLevelFlag(app)

	inputDir  = app.Flag("dir", "A directory containing a .RAW image or a rootfs directory").String()
	outputDir = app.Flag("output-dir", "A destination directory for the output image").String()

	configFile = app.Flag("config", "Path to the image config file.").Required().ExistingFile()
	tmpDir     = app.Flag("tmp-dir", "Directory to store temporary files while converting.").String()

	releaseVersion = app.Flag("release-version", "Release version to add to the output artifact name").String()

	workers = app.Flag("workers", "Number of concurrent goroutines to convert with.").Default(defaultWorkerCount).Int()

	imageTag = app.Flag("image-tag", "Tag (text) appended to the image name. Empty by default.").String()
)

func main() {
	app.Version(exe.ToolkitVersion)
	kingpin.MustParse(app.Parse(os.Args[1:]))
	logger.InitBestEffort(*logFile, *logLevel)

	// Set some defaults
	if *inputDir == "" {
		*inputDir = "build"
	}
	if *outputDir == "" {
		*outputDir = "output"
	}
	if *tmpDir == "" {
		*tmpDir = "build"
	}

	if *workers <= 0 {
		logger.Log.Panicf("Value in --workers must be greater than zero. Found %d", *workers)
	}

	inDirPath, err := filepath.Abs(*inputDir)
	if err != nil {
		logger.Log.Panicf("Error when calculating input directory path: %s", err)
	}

	outDirPath, err := filepath.Abs(*outputDir)
	if err != nil {
		logger.Log.Panicf("Error when calculating absolute output path: %s", err)
	}

	tmpDirPath, err := filepath.Abs(*outputDir)
	if err != nil {
		logger.Log.Panicf("Error when calculating absolute temporary path: %s", err)
	}

	err = os.MkdirAll(outDirPath, os.ModePerm)
	if err != nil {
		logger.Log.Panicf("Error when creating output directory. Error: %s", err)
	}

	config, err := configuration.Load(*configFile)
	if err != nil {
		logger.Log.Panicf("Failed loading image configuration. Error: %s", err)
	}

	err = generateImageArtifacts(*workers, inDirPath, outDirPath, *releaseVersion, *imageTag, tmpDirPath, config)
	if err != nil {
		logger.Log.Panic(err)
	}
}

func generateImageArtifacts(workers int, inDir, outDir, releaseVersion, imageTag, tmpDir string, config configuration.Config) (err error) {
	const defaultSystemConfig = 0

	err = os.MkdirAll(tmpDir, os.ModePerm)
	if err != nil {
		return
	}

	if len(config.Disks) > 1 {
		err = fmt.Errorf("this program currently only supports one disk")
		return
	}

	numberOfArtifacts := 0
	for _, disk := range config.Disks {
		numberOfArtifacts += len(disk.Artifacts)
		for _, partition := range disk.Partitions {
			numberOfArtifacts += len(partition.Artifacts)
		}
	}

	logger.Log.Infof("Converting (%d) artifacts", numberOfArtifacts)

	convertRequests := make(chan *convertRequest, numberOfArtifacts)
	convertedResults := make(chan *convertResult, numberOfArtifacts)

	// Start the workers now so they begin working as soon as a new job is buffered.
	for i := 0; i < workers; i++ {
		go artifactConverterWorker(convertRequests, convertedResults, releaseVersion, tmpDir, imageTag, outDir)
	}

	for i, disk := range config.Disks {
		for _, artifact := range disk.Artifacts {
			inputName, isFile := diskArtifactInput(i, disk)
			convertRequests <- &convertRequest{
				inputPath:   filepath.Join(inDir, inputName),
				isInputFile: isFile,
				artifact:    artifact,
			}
		}

		for j, partition := range disk.Partitions {
			for _, artifact := range partition.Artifacts {
				// Currently only process 1 system config
				inputName, isFile := partitionArtifactInput(i, j, &artifact, retrievePartitionSettings(&config.SystemConfigs[defaultSystemConfig], partition.ID))
				convertRequests <- &convertRequest{
					inputPath:   filepath.Join(inDir, inputName),
					isInputFile: isFile,
					artifact:    artifact,
				}
			}
		}
	}

	close(convertRequests)

	failedArtifacts := []string{}
	for i := 0; i < numberOfArtifacts; i++ {
		result := <-convertedResults
		if result.convertedFile == "" {
			failedArtifacts = append(failedArtifacts, result.artifactName)
		} else {
			logger.Log.Infof("[%d/%d] Converted (%s) -> (%s)", (i + 1), numberOfArtifacts, result.originalPath, result.convertedFile)

			// Make a best-effort attempt to update the ownership of the converted file, in case we're
			// being run under sudo
			updateOwnershipOfConvertedFile(result.convertedFile)
		}
	}

	if len(failedArtifacts) != 0 {
		err = fmt.Errorf("failed to generate the following artifacts: %v", failedArtifacts)
	}

	return
}

func updateOwnershipOfConvertedFile(path string) error {
	// If we're not running as root, don't worry about this.
	if os.Geteuid() != 0 {
		return nil
	}

	sudoUidStr := os.Getenv("SUDO_UID")
	sudoGidStr := os.Getenv("SUDO_GID")

	// If we're not clearly running under sudo, don't worry about this either.
	if sudoUidStr == "" || sudoGidStr == "" {
		return nil
	}

	sudoUid, err := strconv.Atoi(sudoUidStr)
	if err != nil {
		return nil
	}

	sudoGid, err := strconv.Atoi(sudoGidStr)
	if err != nil {
		return nil
	}

	logger.Log.Debugf("Running under sudo; moving ownership of %s to %d:%d\n", path, sudoUid, sudoGid)

	err = os.Chown(path, sudoUid, sudoGid)
	if err != nil {
		return err
	}

	return nil
}

func retrievePartitionSettings(systemConfig *configuration.SystemConfig, searchedID string) (foundSetting *configuration.PartitionSetting) {
	for i := range systemConfig.PartitionSettings {
		if systemConfig.PartitionSettings[i].ID == searchedID {
			foundSetting = &systemConfig.PartitionSettings[i]
			return
		}
	}
	logger.Log.Warningf("Couldn't find partition setting '%s' under system config '%s'", searchedID, systemConfig.Name)
	return
}

func artifactConverterWorker(convertRequests chan *convertRequest, convertedResults chan *convertResult, releaseVersion, tmpDir, imageTag, outDir string) {
	const (
		initrdArtifactType = "initrd"
	)

	for req := range convertRequests {
		fullArtifactName := req.artifact.Name

		// Append release version if necessary
		// Note: ISOs creation is a two step process. The first step's initrd artifact type should not append a release version
		// since the release version value could change between the end of the first step and the start of the second step.
		if req.artifact.Type != initrdArtifactType {
			if releaseVersion != "" {
				fullArtifactName = fullArtifactName + "-" + releaseVersion
			}
		}
		result := &convertResult{
			artifactName: fullArtifactName,
			originalPath: req.inputPath,
		}

		workingArtifactPath := req.inputPath
		isInputFile := req.isInputFile

		if req.artifact.Type != "" {
			const appendExtension = false
			outputFile, err := convertArtifact(fullArtifactName, tmpDir, req.artifact.Type, imageTag, workingArtifactPath, isInputFile, appendExtension)
			if err != nil {
				logger.Log.Errorf("Failed to convert artifact (%s) to type (%s). Error: %s", req.artifact.Name, req.artifact.Type, err)
				convertedResults <- result
				continue
			}
			isInputFile = true
			workingArtifactPath = outputFile
		}

		if req.artifact.Compression != "" {
			const appendExtension = true
			outputFile, err := convertArtifact(fullArtifactName, tmpDir, req.artifact.Compression, imageTag, workingArtifactPath, isInputFile, appendExtension)
			if err != nil {
				logger.Log.Errorf("Failed to compress (%s) using (%s). Error: %s", workingArtifactPath, req.artifact.Compression, err)
				convertedResults <- result
				continue
			}
			workingArtifactPath = outputFile
		}

		if workingArtifactPath == req.inputPath {
			logger.Log.Errorf("Artifact (%s) has no type or compression", req.artifact.Name)
		} else {
			finalFile := filepath.Join(outDir, filepath.Base(workingArtifactPath))
			err := file.Move(workingArtifactPath, finalFile)
			if err != nil {
				logger.Log.Errorf("Failed to move (%s) to (%s). Error: %s", workingArtifactPath, finalFile, err)
			} else {
				result.convertedFile = finalFile
			}
		}

		convertedResults <- result
	}
}

func convertArtifact(artifactName, outDir, format, imageTag, input string, isInputFile, appendExtension bool) (outputFile string, err error) {
	typeConverter, err := formats.ConverterFactory(format)
	if err != nil {
		return
	}

	var originalExt string

	if appendExtension {
		originalExt = path.Ext(input)
	}

	newExt := fmt.Sprintf(".%s", typeConverter.Extension())
	if originalExt != "" {
		newExt = fmt.Sprintf("%s%s", originalExt, newExt)
	}

	if imageTag != "" {
		imageTag = "-" + imageTag
	}

	outputPath := filepath.Join(outDir, artifactName)
	outputFile = fmt.Sprintf("%s%s%s", outputPath, imageTag, newExt)

	err = typeConverter.Convert(input, outputFile, isInputFile)
	return
}

func diskArtifactInput(diskIndex int, disk configuration.Disk) (input string, isFile bool) {
	const rootfsPrefix = "rootfs"

	// If there are no paritions, this is a rootfs
	if len(disk.Partitions) == 0 {
		input = rootfsPrefix
	} else {
		input = fmt.Sprintf("disk%d.raw", diskIndex)
		isFile = true
	}

	return
}

func partitionArtifactInput(diskIndex, partitionIndex int, diskPartArtifact *configuration.Artifact, partitionSetting *configuration.PartitionSetting) (input string, isFile bool) {
	// Currently all file artifacts have a raw file for input
	if diskPartArtifact.Type == "diff" && partitionSetting.OverlayBaseImage != "" {
		input = fmt.Sprintf("disk%d.partition%d.diff", diskIndex, partitionIndex)
	} else if diskPartArtifact.Type == "rdiff" && partitionSetting.RdiffBaseImage != "" {
		input = fmt.Sprintf("disk%d.partition%d.rdiff", diskIndex, partitionIndex)
	} else {
		input = fmt.Sprintf("disk%d.partition%d.raw", diskIndex, partitionIndex)
	}
	isFile = true
	return
}
