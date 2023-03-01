// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// A tool for booting built images

package main

import (
	"fmt"
	"io"
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

	"github.com/microsoft/CBL-Mariner/toolkit/tools/embeddedassets"
	"github.com/microsoft/CBL-Mariner/toolkit/tools/imagegen/configuration"
	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/exe"
	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/logger"

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

	backend = app.Flag("backend", "Backend to use").Default("qemu").Enum("qemu", "libvirt")
)

func main() {
	app.Version(exe.ToolkitVersion)
	kingpin.MustParse(app.Parse(os.Args[1:]))
	logger.InitBestEffort(*logFile, *logLevel)

	// Set up defaults.
	if *imageDir == "" {
		*imageDir = "."
	}

	// Validate
	if *ssh && *backend != "libvirt" {
		logger.Log.Panicf("--ssh requires using libvirt as a backend")
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
	logger.Log.Debugf("Booting image...")

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
			fileExtension = artifact.Type
		}

		imagePath = fmt.Sprintf("%s/%s.%s", inDir, artifact.Name, fileExtension)
	}

	logger.Log.Debugf("Using image: %s", imagePath)

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
	logger.Log.Debugf("Cloning image for ephemeral usage...")

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
	logger.Log.Debugf("Building meta user data iso...")

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

	userDataData, err := embeddedassets.Assets.ReadFile(filepath.Join(embeddedassets.Root, "meta-user-data/user-data"))
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

	metaDataData, err := embeddedassets.Assets.ReadFile(filepath.Join(embeddedassets.Root, "meta-user-data/meta-data"))
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
		"/usr/share/OVMF/OVMF_CODE_4M.ms.fd",
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
		"/usr/share/OVMF/OVMF_VARS_4M.ms.fd",
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
	logger.Log.Debugf("Booting UEFI image...")

	vmChan := make(chan error)
	ipAddrChan := make(chan string)
	stopChan := make(chan interface{})

	vmName := fmt.Sprintf("mariner-%s", uuid.New().String())

	go runUefiVm(imagePath, metaUserDataImagePath, enableGui, ssh, vmName, vmChan)

	if ssh {
		go findIpAddressForVm(ssh, vmName, ipAddrChan, stopChan)
	}

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
	if ssh {
		close(stopChan)
		<-ipAddrChan
	}

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

func findIpAddressForVm(waitForSsh bool, vmName string, ipAddrChan chan string, stopChan chan interface{}) {
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
			ipAddr = tryFindIpAddressForVm(vmName)
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

func tryFindIpAddressForVm(vmName string) string {
	if *backend != "libvirt" {
		logger.Log.Debugf("IP address finding requires libvirt")
		return ""
	}

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

func runUefiVm(imagePath, metaUserDataImagePath string, enableGui, ssh bool, vmName string, c chan error) {
	defer close(c)

	logger.Log.Debugf("Running UEFI VM...")

	cmd, nvramPath, err := launchUefiVm(imagePath, metaUserDataImagePath, enableGui, ssh, vmName)

	if nvramPath != "" {
		defer os.Remove(nvramPath)
	}

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

func launchUefiVm(imagePath, metaUserDataImagePath string, enableGui, ssh bool, vmName string) (cmd *exec.Cmd, nvramPath string, err error) {
	const (
		guestRam        = 1024
		guestVcpus      = 2
		guestOsInfo     = "linux2020"
		guestNoGraphics = "none"
	)

	logger.Log.Debugf("Launching UEFI VM...")

	var requiredTool string
	if *backend == "libvirt" {
		requiredTool = "virt-install"
	} else {
		requiredTool = "qemu-system-x86_64"
	}

	_, err = exec.LookPath(requiredTool)
	if err != nil {
		logger.Log.Errorf("this program requires '%s' and its dependencies to be installed", requiredTool)
		return
	}

	loaderPath, err := findLoaderForUefiVm()
	if err != nil {
		return nil, "", err
	}

	nvramTemplatePath, err := findNvramTemplateForUefiVm()
	if err != nil {
		return nil, "", err
	}

	nvramPath, err = createEmptyTempFile()
	if err != nil {
		return
	}

	var args []string
	if *backend == "libvirt" {
		os.Remove(nvramPath)

		args = []string{
			requiredTool,
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
		}

		if !enableGui {
			args = append(args, "--graphics", guestNoGraphics)

			if ssh {
				args = append(args, "--noautoconsole", "--wait")
			}
		}
	} else {
		nvramTemplateFile, err := os.Open(nvramTemplatePath)
		if err != nil {
			logger.Log.Errorf("Failed to open NVRAM template")
			return nil, nvramPath, err
		}

		defer nvramTemplateFile.Close()

		nvramFile, err := os.OpenFile(nvramPath, os.O_RDWR, 0660)
		if err != nil {
			logger.Log.Errorf("Failed to open NVRAM temporary file: %v", err)
			return nil, nvramPath, err
		}

		defer nvramFile.Close()

		_, err = io.Copy(nvramFile, nvramTemplateFile)
		if err != nil {
			logger.Log.Errorf("Failed to copy NVRAM template")
			return nil, nvramPath, err
		}

		err = nvramFile.Close()
		if err != nil {
			logger.Log.Errorf("Failed to close NVRAM file")
			return nil, nvramPath, err
		}

		imageFormat := "raw"

		switch filepath.Ext(imagePath) {
		case ".qcow2":
			imageFormat = "qcow2"
		case ".raw":
			imageFormat = "raw"
		default:
			logger.Log.Errorf("Unsupported image: %s", imagePath)
			return nil, nvramPath, err
		}

		args = []string{
			requiredTool,
			"-enable-kvm",
			"-machine", "q35,smm=on",
			"-cpu", "host",
			"-smp", fmt.Sprintf("cores=%d,threads=1", guestVcpus),
			"-m", fmt.Sprintf("%dM", guestRam),
			"-object", "rng-random,filename=/dev/urandom,id=rng0",
			"-device", "virtio-rng-pci,rng=rng0",
			"-global", "driver=cfi.pflash01,property=secure,value=on",
			"-drive", fmt.Sprintf("if=pflash,format=raw,unit=0,file=%s,readonly=on", loaderPath),
			"-drive", fmt.Sprintf("if=pflash,format=raw,unit=1,file=%s", nvramPath),
			"-drive", fmt.Sprintf("if=none,id=hd,file=%s,format=%s", imagePath, imageFormat),
			"-device", "virtio-scsi-pci,id=scsi",
			"-device", "scsi-hd,drive=hd,bootindex=1",
			"-cdrom", metaUserDataImagePath,
		}

		if !enableGui {
			args = append(args, "-nographic", "-serial", "mon:stdio")
		}
	}

	logger.Log.Debugf("Launching VM: %s\n", args)

	cmd = exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err = cmd.Start()
	if err != nil {
		return nil, nvramPath, fmt.Errorf("vm process exited with error: %v", err)
	}

	return cmd, nvramPath, nil
}
