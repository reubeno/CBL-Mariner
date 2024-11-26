// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package build

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/configuration"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/spf13/cobra"
)

type bootOptions struct {
	dryRun      bool
	imageConfig string
}

var options bootOptions

var bootCmd = &cobra.Command{
	Use:   "boot",
	Short: "Boot Azure Linux images",
	RunE: func(c *cobra.Command, args []string) error {
		return bootImage(cmd.CmdEnv)
	},
	SilenceUsage: true,
}

func init() {
	cmd.RootCmd.AddCommand(bootCmd)

	bootCmd.Flags().BoolVar(&options.dryRun, "dry-run", false, "Prepare build environment but do not build")
	bootCmd.Flags().StringVarP(&options.imageConfig, "config", "c", "", "Path to the image config file")
}

func bootImage(env *cmd.BuildEnv) error {
	configFilePath, err := env.ResolveImageConfig(options.imageConfig)
	if err != nil {
		return err
	}

	configName := strings.TrimSuffix(filepath.Base(configFilePath), ".json")

	config, err := configuration.Load(configFilePath)
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

	return bootImageUsingDiskFile(imagePath, artifact.Type, artifact.Compression, systemConfig.BootType, options.dryRun)
}

func bootImageUsingDiskFile(imagePath, artifactType, compressionType, bootType string, dryRun bool) error {
	if bootType != "efi" {
		return fmt.Errorf("only EFI boot is supported")
	}

	if compressionType != "" {
		return fmt.Errorf("compressed images are not supported")
	}

	const fwPath = "/usr/share/OVMF/OVMF_CODE.fd"
	const nvramTemplatePath = "/usr/share/OVMF/OVMF_VARS.fd"

	tempDir, err := os.MkdirTemp(os.TempDir(), "azl")
	if err != nil {
		return err
	}

	defer os.RemoveAll(tempDir)

	nvramPath := path.Join(tempDir, "nvram.bin")

	err = file.Copy(nvramTemplatePath, nvramPath)
	if err != nil {
		return err
	}

	qemuArgs := []string{
		"-enable-kvm",
		"-machine", "q35,smm=on",
		"-cpu", "host",
		"-smp", "cores=8,threads=1",
		"-m", "4G",
		"-object", "rng-random,filename=/dev/urandom,id=rng0",
		"-device", "virtio-rng-pci,rng=rng0",
		"-global", "driver=cfi.pflash01,property=secure,value=off",
		"-drive", fmt.Sprintf("if=pflash,format=raw,unit=0,file=%s,readonly=on", fwPath),
		"-drive", fmt.Sprintf("if=pflash,format=raw,unit=1,file=%s", nvramPath),
		"-drive", fmt.Sprintf("if=none,id=hd,file=%s,format=%s", imagePath, artifactType),
		"-device", "virtio-scsi-pci,id=scsi",
		"-device", "scsi-hd,drive=hd,bootindex=1",
		"-netdev", "user,id=n1,hostfwd=tcp::5555-:22",
		"-device", "virtio-net-pci,netdev=n1",
		"-nographic",
		"-serial", "mon:stdio",
	}

	qemuCmd := exec.Command("qemu-system-x86_64", qemuArgs...)
	qemuCmd.Stdout = os.Stdout
	qemuCmd.Stderr = os.Stderr
	qemuCmd.Stdin = os.Stdin

	if dryRun {
		slog.Info("Dry run; would launch VM using qemu", "command", qemuCmd)
		return nil
	}

	return qemuCmd.Run()
}
