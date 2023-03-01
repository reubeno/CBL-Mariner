package gogetrpm

import (
	"archive/tar"
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/cavaliergopher/cpio"
	"github.com/cavaliergopher/rpm"

	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/logger"
)

type repomdData struct {
	XMLName  xml.Name          `xml:"repomd"`
	Revision uint64            `xml:"revision,attr"`
	Data     []repomdDataEntry `xml:"data"`
}

type repomdDataEntry struct {
	XMLName      xml.Name       `xml:"data"`
	Type         string         `xml:"type,attr"`
	Checksum     repomdChecksum `xml:"checksum"`
	OpenChecksum repomdChecksum `xml:"open-checksum"`
	Location     repomdLocation `xml:"location"`
	Timestamp    uint64         `xml:"timestamp"`
	Size         uint64         `xml:"size"`
	OpenSize     uint64         `xml:"open-size"`
}

type repomdChecksum struct {
	Type     string `xml:"type,attr"`
	Checksum string `xml:",chardata"`
}

type repomdLocation struct {
	XMLName xml.Name `xml:"location"`
	Href    string   `xml:"href,attr"`
}

type repoPackageMetadata struct {
	XMLName      xml.Name          `xml:"metadata"`
	PackageCount uint32            `xml:"packages,attr"`
	Packages     []packageMetadata `xml:"package"`
}

type packageMetadata struct {
	XMLName     xml.Name        `xml:"package"`
	Type        string          `xml:"type,attr"`
	Name        string          `xml:"name"`
	Arch        string          `xml:"arch"`
	Version     packageVersion  `xml:"version"`
	Checksum    packageChecksum `xml:"checksum"`
	Summary     string          `xml:"summary"`
	Description string          `xml:"description"`
	Packager    string          `xml:"packager"`
	Url         string          `xml:"url"`
	Time        packageTime     `xml:"time"`
	Size        packageSize     `xml:"size"`
	Location    repomdLocation  `xml:"location"`
	Format      packageFormat   `xml:"format"`
}

type packageVersion struct {
	XMLName xml.Name `xml:"version"`
	Epoch   string   `xml:"epoch,attr"`
	Ver     string   `xml:"ver,attr"`
	Rel     string   `xml:"rel,attr"`
}

type packageChecksum struct {
	XMLName  xml.Name `xml:"checksum"`
	Type     string   `xml:"type,attr"`
	PkgId    string   `xml:"pkgid,attr"`
	Checksum string   `xml:",chardata"`
}

type packageTime struct {
	XMLName xml.Name `xml:"time"`
	File    uint64   `xml:"file,attr"`
	Build   uint64   `xml:"build,attr"`
}

type packageSize struct {
	XMLName   xml.Name `xml:"size"`
	Package   uint64   `xml:"package,attr"`
	Installed uint64   `xml:"installed,attr"`
	Archive   uint64   `xml:"archive,attr"`
}

type packageFormat struct {
	XMLName     xml.Name           `xml:"format"`
	License     string             `xml:"license"`
	Vendor      string             `xml:"vendor"`
	Group       string             `xml:"group"`
	BuildHost   string             `xml:"buildhost"`
	SourceRpm   string             `xml:"sourcerpm"`
	HeaderRange packageHeaderRange `xml:"header-range"`
	Conflicts   []packageEntry     `xml:"conflicts>entry"`
	Enhances    []packageEntry     `xml:"enhances>entry"`
	Obsoletes   []packageEntry     `xml:"obsoletes>entry"`
	Provides    []packageEntry     `xml:"provides>entry"`
	Recommends  []packageEntry     `xml:"recommends>entry"`
	Requires    []packageEntry     `xml:"requires>entry"`
	Suggests    []packageEntry     `xml:"suggests>entry"`
	Supplements []packageEntry     `xml:"supplements>entry"`
	Files       []packageFile      `xml:"file"`
}

type packageHeaderRange struct {
	XMLName xml.Name `xml:"header-range"`
	Start   uint64   `xml:"start,attr"`
	End     uint64   `xml:"end,attr"`
}

type packageEntry struct {
	XMLName xml.Name `xml:"entry"`
	Name    string   `xml:"name,attr"`
	Flags   string   `xml:"flags,attr"`
	Epoch   string   `xml:"epoch,attr"`
	Ver     string   `xml:"ver,attr"`
	Rel     string   `xml:"rel,attr"`
	Pre     string   `xml:"pre,attr"`
}

type packageFile struct {
	XMLName xml.Name `xml:"file"`
	Type    string   `xml:"type,attr"`
	Path    string   `xml:",chardata"`
}

type packageInfo struct {
	metadata packageMetadata
	repoUri  string
}

func BuildTdnfWorkerTarball(repoUris []string, packageNames []string, tarballPath string) error {
	allPackages := make(map[string]packageInfo)

	// Enumerate URIs in reverse, since addPackagesInRepo goes with last-package-wins strategy
	// for duplicates.
	for i := len(repoUris) - 1; i >= 0; i-- {
		repoUri := repoUris[i]
		err := addPackagesInRepo(allPackages, repoUri)
		if err != nil {
			return err
		}
	}

	selectedPackages, err := computeDependencyClosure(allPackages, packageNames)
	if err != nil {
		return err
	}

	err = createTarballFromPackages(allPackages, selectedPackages, tarballPath)
	if err != nil {
		return err
	}

	fi, err := os.Stat(tarballPath)
	if err != nil {
		return err
	}

	logger.Log.Debugf("Done: tarball is %.2f MiB", float64(fi.Size())/1024./1024.)

	return nil
}

func retrieveFile(uri string) (io.ReadCloser, error) {
	parsedUri, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URI %s; err: %v", uri, err)
	}

	if parsedUri.Scheme == "file" {
		if !path.IsAbs(parsedUri.Path) {
			return nil, fmt.Errorf("bad local URI: %s", uri)
		}

		file, err := os.Open(parsedUri.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to open local file %s; err: %v", parsedUri.Path, err)
		}

		return file, nil
	} else {
		res, err := http.Get(uri)
		if err != nil {
			return nil, err
		}

		if res.StatusCode != 200 {
			return nil, fmt.Errorf("failed to retrieve repo metadata; status code: %v", res.StatusCode)
		}

		return res.Body, nil
	}
}

func addPackagesInRepo(packages map[string]packageInfo, repoUri string) error {
	logger.Log.Debugf("Connecting to package feed...\n")
	repomdUri := repoUri + "/repodata/repomd.xml"

	repomdFile, err := retrieveFile(repomdUri)
	if err != nil {
		return err
	}

	defer repomdFile.Close()

	bytes, err := io.ReadAll(repomdFile)
	if err != nil {
		return err
	}

	var repomd repomdData
	err = xml.Unmarshal(bytes, &repomd)
	if err != nil {
		return err
	}

	primaryHref := ""
	for _, data := range repomd.Data {
		if data.Type == "primary" {
			primaryHref = data.Location.Href
			break
		}
	}

	if primaryHref == "" {
		return fmt.Errorf("couldn't find primary repo data: %v", err)
	}

	primaryUri := repoUri + "/" + primaryHref

	logger.Log.Debugf("Retrieving package metadata...\n")

	primaryFile, err := retrieveFile(primaryUri)
	if err != nil {
		return err
	}

	defer primaryFile.Close()

	decompressingReader, err := gzip.NewReader(primaryFile)
	if err != nil {
		return err
	}

	primaryBytes, err := io.ReadAll(decompressingReader)
	if err != nil {
		return err
	}

	var repoPackageMeta repoPackageMetadata
	err = xml.Unmarshal(primaryBytes, &repoPackageMeta)
	if err != nil {
		return err
	}

	logger.Log.Debugf("Found %d package(s) at %s\n", repoPackageMeta.PackageCount, repoUri)

	for _, pkg := range repoPackageMeta.Packages {
		// Last one wins
		packages[pkg.Name] = packageInfo{
			metadata: pkg,
			repoUri:  repoUri,
		}
	}

	return nil
}

func computeDependencyClosure(allPackages map[string]packageInfo, roots []string) ([]string, error) {
	provisions := make(map[string]packageInfo)
	for _, pkg := range allPackages {
		for _, entry := range pkg.metadata.Format.Provides {
			provisions[entry.Name] = pkg
		}

		for _, file := range pkg.metadata.Format.Files {
			provisions[file.Path] = pkg
		}
	}

	logger.Log.Debugf("Resolving package dependencies...\n")

	includedPkgs := make(map[string]bool)
	for _, pkgName := range roots {
		includedPkgs[pkgName] = true

		pkg := allPackages[pkgName]
		if pkg.metadata.Name == "" {
			return nil, fmt.Errorf("can't find package: %s", pkgName)
		}

		// TODO: Match more than just name.
		for _, req := range pkg.metadata.Format.Requires {
			if provisions[req.Name].metadata.Name != "" {
				includedPkgs[provisions[req.Name].metadata.Name] = true
			} else {
				return nil, fmt.Errorf("can't find requirement: %s", req.Name)
			}
		}
	}

	logger.Log.Debugf("Resolved full set of %d required packages.\n", len(includedPkgs))

	var includedPkgNames []string
	for name := range includedPkgs {
		includedPkgNames = append(includedPkgNames, name)
	}

	return includedPkgNames, nil
}

func downloadPackage(uri, filename string) error {
	res, err := http.Get(uri)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	if res.StatusCode != 200 {
		return fmt.Errorf("failed to download %s; status code: %v", uri, res.StatusCode)
	}

	out, err := os.Create(filename)
	if err != nil {
		return err
	}

	defer out.Close()

	_, err = io.Copy(out, res.Body)
	if err != nil {
		return err
	}

	return nil
}

func createTarballFromPackages(allPackages map[string]packageInfo, selectedPackages []string, outputPath string) error {
	// Start creating the tarball
	tarOut, err := os.Create(outputPath)
	if err != nil {
		return err
	}

	defer tarOut.Close()
	gzipWriter := gzip.NewWriter(tarOut)
	defer gzipWriter.Close()
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Setup initial links
	err = setupInitialDirsInTarball(tarWriter)
	if err != nil {
		return err
	}

	// Import the packages' content
	totalFileCount := 0
	for _, pkgName := range selectedPackages {
		pkg := allPackages[pkgName]
		pkgUri := pkg.repoUri + "/" + pkg.metadata.Location.Href

		logger.Log.Debugf("importing: %s\n", pkgUri)

		fileCount, err := importPackageIntoTarball(pkgUri, tarWriter)
		if err != nil {
			return err
		}

		totalFileCount += fileCount
	}

	logger.Log.Debugf("Created tarball with %d file(s).", totalFileCount)

	return nil
}

func setupInitialDirsInTarball(tarWriter *tar.Writer) error {
	type Link struct {
		name   string
		target string
	}

	dirs := []string{"./usr", "./usr/sbin", "./usr/bin", "./usr/lib", "./var", "./etc"}
	links := []Link{
		{"./sbin", "usr/sbin"},
		{"./bin", "usr/bin"},
		{"./lib", "usr/lib"},
		{"./lib64", "usr/lib"},
		{"./usr/lib64", "lib"},
		{"./var/run", "../run"},
	}

	for _, dir := range dirs {
		err := addDirToTarball(tarWriter, dir)
		if err != nil {
			return err
		}
	}

	for _, link := range links {
		err := addSymlinkToTarball(tarWriter, link.name, link.target)
		if err != nil {
			return err
		}
	}

	return nil
}

type customFileInfo struct {
	FileName    string
	FileSize    int64
	FileMode    os.FileMode
	FileModTime time.Time
}

func (i *customFileInfo) Name() string {
	return i.FileName
}

func (i *customFileInfo) IsDir() bool {
	return i.FileMode.IsDir()
}

func (i *customFileInfo) ModTime() time.Time {
	return i.FileModTime
}

func (i *customFileInfo) Mode() fs.FileMode {
	return i.FileMode
}

func (i *customFileInfo) Size() int64 {
	return i.FileSize
}

func (i *customFileInfo) Sys() interface{} {
	return nil
}

func addDirToTarball(tarWriter *tar.Writer, dirName string) error {
	var fi customFileInfo
	fi.FileName = path.Base(dirName)
	fi.FileMode = fs.ModeDir | 0755
	fi.FileModTime = time.Now()

	hdr, err := tar.FileInfoHeader(&fi, "" /*link*/)
	if err != nil {
		return err
	}

	hdr.Name = dirName

	err = tarWriter.WriteHeader(hdr)
	if err != nil {
		return err
	}

	return nil
}

func addSymlinkToTarball(tarWriter *tar.Writer, linkName, target string) error {
	var fi customFileInfo
	fi.FileName = path.Base(linkName)
	fi.FileMode = fs.ModeSymlink | 0777
	fi.FileModTime = time.Now()

	hdr, err := tar.FileInfoHeader(&fi, target)
	if err != nil {
		return err
	}

	hdr.Name = linkName

	err = tarWriter.WriteHeader(hdr)
	if err != nil {
		return err
	}

	return nil
}

func importPackageIntoTarball(packageUri string, tarWriter *tar.Writer) (int, error) {
	packageFile, err := retrieveFile(packageUri)
	if err != nil {
		return 0, err
	}

	defer packageFile.Close()

	fileCount, err := importPackageIntoTarballFromReader(packageFile, tarWriter)
	if err != nil {
		return 0, err
	}

	return fileCount, nil
}

func importPackageIntoTarballFromReader(packageFile io.Reader, tarWriter *tar.Writer) (int, error) {
	// Read the package headers
	pkg, err := rpm.Read(packageFile)
	if err != nil {
		return 0, err
	}

	// Check the compression algorithm of the payload
	if compression := pkg.PayloadCompression(); compression != "gzip" {
		return 0, fmt.Errorf("unsupported compression: %s", compression)
	}

	// Attach a reader to decompress the payload
	gzipReader, err := gzip.NewReader(packageFile)
	if err != nil {
		return 0, err
	}

	// Check the archive format of the payload
	if format := pkg.PayloadFormat(); format != "cpio" {
		return 0, fmt.Errorf("unsupported payload format: %s", format)
	}

	// Attach a reader to unarchive each file in the payload
	count := 0
	cpioReader := cpio.NewReader(gzipReader)
	for {
		// Move to the next file in the archive
		fileInCpio, err := cpioReader.Next()
		if err == io.EOF {
			break // no more files
		}
		if err != nil {
			return 0, err
		}

		err = importFileIntoTarball(tarWriter, cpioReader, fileInCpio)
		if err != nil {
			return 0, err
		}

		count++
	}

	return count, nil
}

func importFileIntoTarball(tarWriter *tar.Writer, cpioReader *cpio.Reader, fileInCpio *cpio.Header) error {
	cpioFileInfo := fileInCpio.FileInfo()

	linkTarget := ""
	if cpioFileInfo.Mode().Type() == fs.ModeSymlink {
		linkTarget = fileInCpio.Linkname
	}

	tarFileInfo, err := tar.FileInfoHeader(cpioFileInfo, linkTarget)
	if err != nil {
		return err
	}

	tarFileInfo.Name = fileInCpio.Name

	err = tarWriter.WriteHeader(tarFileInfo)
	if err != nil {
		return err
	}

	_, err = io.Copy(tarWriter, cpioReader)
	if err != nil {
		return err
	}

	return nil
}
