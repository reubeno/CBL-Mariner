// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package build

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/utils"
	"github.com/spf13/cobra"
)

type bootOptions struct {
	authorizedPublicKeyPath string
	dryRun                  bool
	useDiskRW               bool
	imageConfig             string
	secureBoot              bool
	testUserName            string
	testUserPassword        string
	workDir                 string
}

var options bootOptions

var bootCmd = &cobra.Command{
	Use:   "boot",
	Short: "Boot Azure Linux images",
	RunE: func(c *cobra.Command, args []string) error {
		// Set up default for work dir
		if options.workDir == "" {
			options.workDir = path.Join(cmd.CmdEnv.RepoRootDir, "artifacts")
			err := os.MkdirAll(options.workDir, 0755)
			if err != nil {
				return err
			}
		}

		// Now boot.
		return bootImage(cmd.CmdEnv)
	},
	SilenceUsage: true,
}

func init() {
	cmd.RootCmd.AddCommand(bootCmd)

	bootCmd.Flags().BoolVar(&options.dryRun, "dry-run", false, "Prepare build environment but do not build")

	bootCmd.Flags().StringVarP(&options.imageConfig, "config", "c", "", "Path to the image config file")
	bootCmd.MarkFlagRequired("config")

	bootCmd.Flags().StringVar(&options.testUserName, "test-user", "test", "Name for the test account (defaults to test)")
	bootCmd.Flags().StringVar(&options.testUserPassword, "test-password", "", "Password for the test account")
	bootCmd.MarkFlagRequired("test-password")

	bootCmd.Flags().StringVar(&options.authorizedPublicKeyPath, "authorized-public-key", "", "Path to public key authorized for SSH to test user account")
	bootCmd.MarkFlagFilename("authorized-public-key")

	bootCmd.Flags().StringVar(&options.workDir, "work-dir", "", "Directory to use for temporary files (may include large disk images)")
	bootCmd.MarkFlagDirname("work-dir")

	bootCmd.Flags().BoolVar(&options.useDiskRW, "rwdisk", false, "Allow writes to persist to the disk image")
	bootCmd.Flags().BoolVar(&options.secureBoot, "secure-boot", false, "Enable secure boot for the VM")
}

func bootImage(env *cmd.BuildEnv) error {
	configFilePath, err := env.ResolveImageConfig(options.imageConfig)
	if err != nil {
		return err
	}

	configName := strings.TrimSuffix(filepath.Base(configFilePath), ".json")

	config, err := utils.ParseImageConfig(configFilePath)
	if err != nil {
		return err
	}

	if len(config.SystemConfigs) != 1 {
		return fmt.Errorf("expected exactly one system configuration in the image configuration")
	}

	systemConfig := &config.SystemConfigs[0]

	if len(config.Disks) != 1 {
		return fmt.Errorf("expected exactly one disk in the image configuration")
	}

	disk := &config.Disks[0]

	if len(disk.Artifacts) != 1 {
		return fmt.Errorf("expected exactly one artifact in the disk configuration")
	}

	artifact := &disk.Artifacts[0]

	pattern := path.Join(env.ImageOutputDir, configName, artifact.Name+"*."+artifact.Type)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}

	sort.Strings(matches)

	if len(matches) == 0 {
		return fmt.Errorf("no matching image files found")
	}

	imagePath := matches[len(matches)-1]

	return bootImageUsingDiskFile(imagePath, artifact.Type, artifact.Compression, systemConfig.BootType, options.secureBoot, options.useDiskRW, options.dryRun, options.workDir)
}

func bootImageUsingDiskFile(imagePath, artifactType, compressionType, bootType string, secureBoot, rwDisk, dryRun bool, workDir string) error {
	if bootType != "efi" {
		return fmt.Errorf("only EFI boot is supported")
	}

	if compressionType != "" {
		return fmt.Errorf("compressed images are not supported")
	}

	fwPath, nvramTemplatePath, err := findVmFirmware(secureBoot)
	if err != nil {
		return nil
	}

	tempDir, err := os.MkdirTemp(workDir, "azl")
	if err != nil {
		return err
	}

	defer os.RemoveAll(tempDir)

	nvramPath := path.Join(tempDir, "nvram.bin")

	err = copyFile(nvramTemplatePath, nvramPath)
	if err != nil {
		return err
	}

	cloudInitMetadataIsoPath := path.Join(tempDir, "cloud-init.iso")

	err = buildCloudInitMetadataIso(options, cloudInitMetadataIsoPath, dryRun, workDir)
	if err != nil {
		return err
	}

	var selectedDiskPath string
	var selectedDiskType string
	if !rwDisk {
		selectedDiskType = artifactType
		selectedDiskPath = path.Join(tempDir, "ephemeral.img")

		err = copyFile(imagePath, selectedDiskPath)
		if err != nil {
			return err
		}
	} else {
		selectedDiskPath = imagePath
		selectedDiskType = artifactType
	}

	var secureBootOnOff string
	if secureBoot {
		secureBootOnOff = "on"
	} else {
		secureBootOnOff = "off"
	}

	qemuArgs := []string{
		"qemu-system-x86_64",
		"-enable-kvm",
		"-machine", "q35,smm=on",
		"-cpu", "host",
		"-smp", "cores=8,threads=1",
		"-m", "4G",
		"-object", "rng-random,filename=/dev/urandom,id=rng0",
		"-device", "virtio-rng-pci,rng=rng0",
		"-global", fmt.Sprintf("driver=cfi.pflash01,property=secure,value=%s", secureBootOnOff),
		"-drive", fmt.Sprintf("if=pflash,format=raw,unit=0,file=%s,readonly=on", fwPath),
		"-drive", fmt.Sprintf("if=pflash,format=raw,unit=1,file=%s", nvramPath),
		"-drive", fmt.Sprintf("if=none,id=hd,file=%s,format=%s", selectedDiskPath, selectedDiskType),
		"-device", "virtio-scsi-pci,id=scsi",
		"-device", "scsi-hd,drive=hd,bootindex=1",
		"-cdrom", cloudInitMetadataIsoPath,
		"-netdev", "user,id=n1,hostfwd=tcp::8888-:22",
		"-device", "virtio-net-pci,netdev=n1",
		"-nographic",
		"-serial", "mon:stdio",
	}

	qemuCmd := exec.Command("sudo", qemuArgs...)
	qemuCmd.Stdout = os.Stdout
	qemuCmd.Stderr = os.Stderr
	qemuCmd.Stdin = os.Stdin

	if dryRun {
		slog.Info("Dry run; would launch VM using qemu", "command", qemuCmd)
		return nil
	}

	return qemuCmd.Run()
}

func findVmFirmware(secureBoot bool) (fwPath, nvramTemplatePath string, err error) {
	var fwPaths []string
	var nvramTemplatePaths []string
	if secureBoot {
		fwPaths = []string{
			"/usr/share/OVMF/OVMF_CODE.secboot.fd",
			"/usr/share/OVMF/OVMF_CODE_4M.secboot.fd",
		}
		nvramTemplatePaths = []string{
			"/usr/share/OVMF/OVMF_VARS.secboot.fd",
			"/usr/share/OVMF/OVMF_VARS_4M.secboot.fd",
		}
	} else {
		fwPaths = []string{
			"/usr/share/OVMF/OVMF_CODE.fd",
			"/usr/share/OVMF/OVMF_CODE_4M.fd",
		}
		nvramTemplatePaths = []string{
			"/usr/share/OVMF/OVMF_VARS.fd",
			"/usr/share/OVMF/OVMF_VARS_4M.fd",
		}
	}

	for _, candidatePath := range fwPaths {
		if _, err := os.Stat(candidatePath); err == nil {
			fwPath = candidatePath
			break
		}
	}

	if fwPath == "" {
		err = fmt.Errorf("OVMF firmware not found")
		return
	}

	for _, candidatePath := range nvramTemplatePaths {
		if _, err := os.Stat(candidatePath); err == nil {
			nvramTemplatePath = candidatePath
			break
		}
	}

	if nvramTemplatePath == "" {
		err = fmt.Errorf("NVRAM template not found")
		return
	}

	return
}

// func convertDiskImage(sourcePath, sourceType, destPath, destType string, dryRun bool) error {
// 	qemuImgCmd := exec.Command("qemu-img", "convert", "-f", sourceType, "-O", destType, sourcePath, destPath)
// 	qemuImgCmd.Stdout = os.Stdout
// 	qemuImgCmd.Stderr = os.Stderr

// 	if dryRun {
// 		slog.Info("Dry run; would convert disk image", "command", qemuImgCmd)
// 		return nil
// 	}

// 	return qemuImgCmd.Run()
// }

func buildCloudInitMetadataIso(options bootOptions, outputFilePath string, dryRun bool, workDir string) error {
	tempDir, err := os.MkdirTemp(workDir, "azl")
	if err != nil {
		return err
	}

	defer os.RemoveAll(tempDir)

	metaDataPath := path.Join(tempDir, "meta-data")
	err = generateCloudInitMetadata(metaDataPath)
	if err != nil {
		return err
	}

	userDataPath := path.Join(tempDir, "user-data")
	err = generateCloudInitUserData(options, userDataPath)
	if err != nil {
		return err
	}

	isoCmd := exec.Command("genisoimage", "-output", outputFilePath, "-volid", "cidata", "-joliet", "-rock", metaDataPath, userDataPath)

	if dryRun {
		slog.Info("Dry run; would create cloud-init metadata ISO", "command", isoCmd)
		return nil
	}

	return isoCmd.Run()
}

func generateCloudInitMetadata(outputFilePath string) error {
	const contents = `
local-hostname: azurelinux-vm
`

	return os.WriteFile(outputFilePath, []byte(contents), 0644)
}

func generateCloudInitUserData(options bootOptions, outputFilePath string) error {
	trueValue := true
	falseValue := false

	testUserConfig := utils.CloudUserConfig{
		Name:                 options.testUserName,
		Description:          "Test User",
		EnableSSHPaswordAuth: &trueValue,
		Shell:                "/bin/bash",
		Sudo:                 []string{"ALL=(ALL) NOPASSWD:ALL"},
		LockPassword:         &falseValue,
		PlainTextPassword:    options.testUserPassword,
		Groups:               []string{"sudo"},
	}

	if options.authorizedPublicKeyPath != "" {
		publicKeyBytes, err := os.ReadFile(options.authorizedPublicKeyPath)
		if err != nil {
			return err
		}

		testUserConfig.SSHAuthorizedKeys = append(testUserConfig.SSHAuthorizedKeys, string(publicKeyBytes))
	}

	detailedConfig := utils.CloudConfig{
		EnableSSHPaswordAuth: &trueValue,
		DisableRootUser:      &trueValue,
		ChangePasswords: &utils.CloudPasswordConfig{
			Expire: &falseValue,
		},
		Users: []utils.CloudUserConfig{
			{
				Name: "default",
			},
			testUserConfig,
		},
	}

	bytes, err := utils.MarshalCloudConfigToYAML(&detailedConfig)
	if err != nil {
		return err
	}

	return os.WriteFile(outputFilePath, bytes, 0644)
}

func copyFile(sourcePath, destPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}

	defer sourceFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return nil
	}

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	err = destFile.Close()
	if err != nil {
		return err
	}

	return nil
}
