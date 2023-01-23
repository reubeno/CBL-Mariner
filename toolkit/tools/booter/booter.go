// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// A tool for booting built images

package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"
	"github.com/google/uuid"

	"github.com/reubeno/CBL-Mariner/toolkit/tools/assets"
	"github.com/reubeno/CBL-Mariner/toolkit/tools/imagegen/configuration"
	"github.com/reubeno/CBL-Mariner/toolkit/tools/internal/exe"
	"github.com/reubeno/CBL-Mariner/toolkit/tools/internal/logger"
	"github.com/reubeno/CBL-Mariner/toolkit/tools/roast/formats"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	app = kingpin.New("booter", "A tool for booting built images")

	logFile  = exe.LogFileFlag(app)
	logLevel = exe.LogLevelFlag(app)

	imageDir = app.Flag("image-dir", "Directory containing built images").ExistingDir()
	tempDir  = app.Flag("temp-dir", "Directory for temporary files").ExistingDir()

	imagePath = app.Flag("image", "Image file path").ExistingFile()

	configFile   = app.Flag("config", "Path to the image config file.").Required().ExistingFile()
	artifactName = app.Flag("artifact", "Name of artifact to boot").String()

	ephemeralStorage = app.Flag("ephemeral-storage", "Discard all writes to storage after terminating").Bool()

	enableGui = app.Flag("gui", "Enable GUI").Bool()
	ssh       = app.Flag("ssh", "ssh to guest").Bool()
)

func main() {
	app.Version(exe.ToolkitVersion)
	kingpin.MustParse(app.Parse(os.Args[1:]))
	logger.InitBestEffort(*logFile, *logLevel)

	// Set up defaults.
	if *imageDir == "" {
		*imageDir = "output"
	}

	inDirPath, err := filepath.Abs(*imageDir)
	if err != nil {
		logger.Log.Panicf("Error when calculating input directory path: %s", err)
	}

	absImagePath := ""
	if *imagePath != "" {
		absImagePath, err = filepath.Abs(*imagePath)
		if err != nil {
			logger.Log.Panicf("Error when calculating image path: %s", err)
		}
	}

	config, err := configuration.Load(*configFile)
	if err != nil {
		logger.Log.Panicf("Failed loading image configuration. Error: %s", err)
	}

	err = bootImage(inDirPath, absImagePath, config, *artifactName, *enableGui, *ssh)
	if err != nil {
		logger.Log.Panic(err)
	}
}

func bootImage(inDir, imagePath string, config configuration.Config, artifactName string, enableGui, ssh bool) (err error) {
	if imagePath == "" {
		if len(config.Disks) != 1 {
			err = fmt.Errorf("this program requires the configuration to have exactly one disk")
			return
		}

		disk := &config.Disks[0]

		if len(disk.Artifacts) == 0 {
			err = fmt.Errorf("no artifacts found in configuration")
			return
		}

		var artifact *configuration.Artifact
		if artifactName == "" {
			if len(disk.Artifacts) != 1 {
				logger.Log.Warnf("this configuration produces multiple artifacts; picking first one ('%s')", disk.Artifacts[0].Name)
			}

			artifact = &disk.Artifacts[0]
		} else {
			for i, candidate := range disk.Artifacts {
				if candidate.Name == artifactName {
					if artifact != nil {
						err = fmt.Errorf("found multiple artifacts named '%s'", artifactName)
						return
					}

					artifact = &disk.Artifacts[i]
				}
			}

			if artifact == nil {
				err = fmt.Errorf("could not find artifact named '%s'", artifactName)
				return
			}
		}

		if len(config.SystemConfigs) != 1 {
			err = fmt.Errorf("this program requires the configuration to have exactly one SystemConfig")
			return
		}

		fileExtension := "raw"
		if artifact.Type != "" {
			converter, err := formats.ConverterFactory(artifact.Type)
			if err != nil {
				return err
			}

			fileExtension = converter.Extension()
		}

		imagePath = fmt.Sprintf("%s/%s.%s", inDir, artifact.Name, fileExtension)
	}

	syscfg := &config.SystemConfigs[0]

	_, err = os.Stat(imagePath)
	if err != nil {
		err = fmt.Errorf("unable to access image: looked at %s; error: %v", imagePath, err)
		return
	}

	metaUserDataImagePath, err := buildMetaUserDataIso()
	if err != nil {
		return fmt.Errorf("unable to build meta-user-data .iso image; error: %v", err)
	}

	defer os.Remove(metaUserDataImagePath)

	if *ephemeralStorage {
		var imageFormat string
		if strings.HasSuffix(imagePath, ".qcow2") {
			imageFormat = "qcow2"
		} else if strings.HasSuffix(imagePath, ".raw") {
			imageFormat = "raw"
		} else {
			return fmt.Errorf("must use --ephemeral-storage with a .raw or .qcow2 image")
		}

		tempImagePath, err := createEphemeralImageBasedOn(imagePath, imageFormat)
		if err != nil {
			return fmt.Errorf("failed to create ephemeral image; error: %v", err)
		}

		defer os.Remove(tempImagePath)

		imagePath = tempImagePath
	}

	switch syscfg.BootType {
	case "efi":
		return bootUefiImage(imagePath, metaUserDataImagePath, enableGui, ssh)
	case "legacy":
		logger.Log.Errorf("not yet implemented for BootType=legacy")
	case "none":
		logger.Log.Errorf("not yet implemented for BootType=none")
	}

	return
}

func createEphemeralImageBasedOn(baseImagePath, baseImageFormat string) (string, error) {
	_, err := exec.LookPath("qemu-img")
	if err != nil {
		return "", fmt.Errorf("--ephemeral-storage requires 'qemu-img' to be in your path")
	}

	tempImageFile, err := os.CreateTemp(*tempDir, "temp-disk-*.qcow2")
	if err != nil {
		return "", fmt.Errorf("failed to find location for ephemeral disk image; error: %v", err)
	}

	tempImagePath := tempImageFile.Name()

	tempImageFile.Close()
	os.Remove(tempImagePath)

	logger.Log.Debugf("Creating temp disk image %s based on %s\n", tempImagePath, baseImagePath)

	cmd := exec.Command(
		"qemu-img",
		"create",
		"-q",
		"-b", baseImagePath,
		"-F", baseImageFormat,
		"-f", "qcow2",
		tempImagePath)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to create ephemeral disk image; error: %v", err)
	}

	return tempImagePath, nil
}

func buildMetaUserDataIso() (string, error) {
	_, err := exec.LookPath("genisoimage")
	if err != nil {
		logger.Log.Errorf("this program requires 'genisoimage' to be in your path")
		return "", err
	}

	isoTempDir, err := os.MkdirTemp(*tempDir, "mariner-iso")
	if err != nil {
		return "", err
	}

	defer os.RemoveAll(isoTempDir)

	userDataFilePath := path.Join(isoTempDir, "user-data")
	userDataFile, err := os.OpenFile(userDataFilePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return "", err
	}

	userDataData, err := assets.Assets.ReadFile("assets/meta-user-data/user-data")
	if err != nil {
		return "", err
	}

	_, err = userDataFile.Write(userDataData)
	if err != nil {
		return "", err
	}

	err = userDataFile.Close()
	if err != nil {
		return "", err
	}

	metaDataFilePath := path.Join(isoTempDir, "meta-data")
	metaDataFile, err := os.OpenFile(metaDataFilePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return "", err
	}

	metaDataData, err := assets.Assets.ReadFile("assets/meta-user-data/meta-data")
	if err != nil {
		return "", err
	}

	_, err = metaDataFile.Write(metaDataData)
	if err != nil {
		return "", err
	}

	err = metaDataFile.Close()
	if err != nil {
		return "", err
	}

	isoFile, err := os.CreateTemp(*tempDir, "meta-user-data-*.iso")
	if err != nil {
		return "", err
	}

	defer isoFile.Close()
	os.Remove(isoFile.Name())

	cmd := exec.Command(
		"genisoimage",
		"-output",
		isoFile.Name(),
		"-volid", "cidata",
		"-joliet",
		"-rock",
		metaDataFilePath, userDataFilePath)

	err = cmd.Run()
	if err != nil {
		os.Remove(isoFile.Name())
		return "", err
	}

	return isoFile.Name(), nil
}

func findLoaderForUefiVm() (string, error) {
	candidates := []string{
		// TODO: Add other potential locations
		"/usr/share/OVMF/OVMF_CODE.secboot.fd",
	}

	for _, candidate := range candidates {
		_, err := os.Stat(candidate)
		if err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("can't find loader for UEFI VM")
}

func findNvramTemplateForUefiVm() (string, error) {
	candidates := []string{
		// TODO: Add other potential locations
		"/usr/share/OVMF/OVMF_VARS.fd",
	}

	for _, candidate := range candidates {
		_, err := os.Stat(candidate)
		if err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("can't find NVRAM template for UEFI VM")
}

func createEmptyTempFile() (path string, err error) {
	file, err := os.CreateTemp(*tempDir, "booter-tmp-")
	if err != nil {
		return "", err
	}

	path = file.Name()
	file.Close()

	return path, nil
}

func bootUefiImage(imagePath, metaUserDataImagePath string, enableGui, ssh bool) (err error) {
	vmChan := make(chan error)
	ipAddrChan := make(chan string)
	stopChan := make(chan interface{})

	vmName := fmt.Sprintf("mariner-%s", uuid.New().String())

	go runUefiVmWithLibVirt(imagePath, metaUserDataImagePath, enableGui, ssh, vmName, vmChan)
	go findIpAddressForLibVirtVm(ssh, vmName, ipAddrChan, stopChan)

	var ipAddr string

outerFor:
	for {
		select {
		case err = <-vmChan:
			break outerFor
		case reportedIpAddr, ok := <-ipAddrChan:
			if ok {
				ipAddr = reportedIpAddr
				logger.Log.Debugf("found IP address on VM: %s\n", ipAddr)
			}
			break outerFor
		}
	}

	// Shutdown IP address finder
	close(stopChan)
	<-ipAddrChan

	// See if VM already shutdown
	if err == nil {
		// TODO: handle not getting IP address
		if ipAddr != "" && ssh {
			runSshClient(ipAddr)

			logger.Log.Debugf("ssh client exited; shutting down VM")

			shutdownErr := exec.Command("virsh", "shutdown", "--domain", vmName).Run()
			if shutdownErr != nil {
				logger.Log.Warnf("VM shutdown request failed: %v\n", shutdownErr)
			}
		}

		// Wait for VM to shutdown
		logger.Log.Debugf("waiting for VM to shutdown")
		vmErr := <-vmChan
		if vmErr != nil && err == nil {
			err = vmErr
		}
	}

	logger.Log.Debugf("VM shutdown")
	return
}

func runSshClient(ipAddr string) error {
	sshArgs := []string{
		"sshpass",
		"-p", "p@ssw0rd",
		"ssh",
		"-q",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("mariner_user@%s", ipAddr),
	}

	logger.Log.Debugf("executing ssh: %v\n", sshArgs)

	sshCmd := exec.Command(sshArgs[0], sshArgs[1:]...)
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr
	sshCmd.Stdin = os.Stdin

	err := sshCmd.Run()
	if err != nil {
		logger.Log.Warnf("ssh client exited with error: %v\n", err)
		return err
	}

	logger.Log.Debugf("ssh client exited\n")
	return nil
}

func findIpAddressForLibVirtVm(waitForSsh bool, vmName string, ipAddrChan chan string, stopChan chan interface{}) {
	defer close(ipAddrChan)

	ipAddr := ""

	for {
		select {
		case <-stopChan:
			return
		default:
			time.Sleep(500 * time.Millisecond)
		}

		if ipAddr == "" {
			ipAddr = tryFindIpAddressForLibVirtVm(vmName)
		}

		if ipAddr == "" {
			continue
		}

		if waitForSsh && !isSshOpenForConnections(ipAddr) {
			continue
		}

		logger.Log.Debugf("confirmed guest ssh daemon is up and running")

		ipAddrChan <- ipAddr
		return
	}
}

func isSshOpenForConnections(ipAddr string) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:22", ipAddr), 10*time.Millisecond)
	if err != nil {
		return false
	}

	conn.Close()
	return true
}

func tryFindIpAddressForLibVirtVm(vmName string) string {
	lv := libvirt.NewWithDialer(dialers.NewLocal())

	err := lv.Connect()
	if err != nil {
		return ""
	}

	domains, _, err := lv.ConnectListAllDomains(1, 8 /*flags: only transient domains*/)
	if err != nil {
		return ""
	}

	var found *libvirt.Domain
	for _, domain := range domains {
		if domain.Name == vmName {
			found = &domain
		}
	}

	if found == nil {
		return ""
	}

	intfs, err := lv.DomainInterfaceAddresses(*found, 0 /*source: parse DHCP leases*/, 0)
	if err != nil {
		return ""
	}

	if len(intfs) == 0 {
		return ""
	}

	if len(intfs) > 1 {
		logger.Log.Warnf("found multiple network interfaces on VM")
	}

	intf := intfs[0]

	if len(intf.Addrs) == 0 {
		return ""
	}

	if len(intf.Addrs) > 1 {
		logger.Log.Warnf("found multiple addresses on VM net interface")
	}

	// TODO: pay attention to address type
	return intf.Addrs[0].Addr
}

func runUefiVmWithLibVirt(imagePath, metaUserDataImagePath string, enableGui, ssh bool, vmName string, c chan error) {
	defer close(c)

	cmd, err := launchUefiVmWithLibVirt(imagePath, metaUserDataImagePath, enableGui, ssh, vmName)
	if err != nil {
		c <- err
		return
	}

	err = cmd.Wait()
	if err != nil {
		c <- fmt.Errorf("vm process exited with error: %v", err)
		return
	}

	logger.Log.Debugf("vm process exited")
	c <- nil
}

func launchUefiVmWithLibVirt(imagePath, metaUserDataImagePath string, enableGui, ssh bool, vmName string) (cmd *exec.Cmd, err error) {
	const (
		guestRam        = 1024
		guestVcpus      = 2
		guestOsInfo     = "linux2020"
		guestNoGraphics = "none"
	)

	_, err = exec.LookPath("virt-install")
	if err != nil {
		logger.Log.Errorf("this program requires 'virt-install' and libvirt dependencies to be installed")
		return
	}

	loaderPath, err := findLoaderForUefiVm()
	if err != nil {
		return nil, err
	}

	nvramTemplatePath, err := findNvramTemplateForUefiVm()
	if err != nil {
		return nil, err
	}

	nvramPath, err := createEmptyTempFile()
	if err != nil {
		return nil, err
	}

	os.Remove(nvramPath)
	defer os.Remove(nvramPath)

	args := []string{
		"virt-install",
		"--transient",
		"--ram", fmt.Sprintf("%d", guestRam),
		"--vcpus", fmt.Sprintf("%d", guestVcpus),
		"--boot", fmt.Sprintf("loader=%s,loader.readonly=yes,loader.secure=no,loader.type=pflash,nvram.template=%s,nvram=%s", loaderPath, nvramTemplatePath, nvramPath),
		"--destroy-on-exit",
		"--import",
		"--disk", imagePath,
		"--osinfo", guestOsInfo,
		"--cdrom", metaUserDataImagePath,
		"-n", vmName,
		"--quiet",
		// "--qemu-commandline=-chardev",
		// "--qemu-commandline=file,id=debuglog,path=/var/lib/libvirt/qemu/debuglog.txt",
		"--qemu-commandline=-debugcon",
		// "--qemu-commandline=chardev:debuglog",
		"--qemu-commandline=stdio",
		"--qemu-commandline=-global",
		"--qemu-commandline=isa-debugcon.iobase=0x402",
	}

	if !enableGui {
		args = append(args, "--graphics", guestNoGraphics)

		if ssh {
			args = append(args, "--noautoconsole", "--wait")
		}
	}

	logger.Log.Debugf("Launching VM: %s\n", args)

	cmd = exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("vm process exited with error: %v", err)
	}

	return cmd, nil
}
