// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package download

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
	"github.com/spf13/cobra"
)

var specPath string
var outputDir string

var downloadSourcesCmd = &cobra.Command{
	Use:   "sources",
	Short: "Download sources for spec",
	RunE: func(c *cobra.Command, args []string) error {
		return downloadSpecSources(specPath, cmd.CmdEnv)
	},
	SilenceUsage: true,
}

type cgmanifest struct {
	Registrations []registration `json:"Registrations"`
	Version       int            `json:"Version"`
}

type registration struct {
	Component component `json:"component"`
}

type component struct {
	ComponentType string `json:"type"`
	Other         other  `json:"other"`
}

type other struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	DownloadUrl string `json:"downloadUrl"`
}

func downloadSpecSources(specPath string, env *cmd.BuildEnv) error {
	slog.Info("Downloading sources", "spec", specPath)

	specFilename := path.Base(specPath)
	specName := strings.TrimSuffix(specFilename, filepath.Ext(specFilename))

	cgmanifestPath := path.Join(env.RepoRootDir, "cgmanifest.json")

	cgmanifestFile, err := os.Open(cgmanifestPath)
	if err != nil {
		return err
	}

	defer cgmanifestFile.Close()

	cgmanifestBytes, err := io.ReadAll(cgmanifestFile)
	if err != nil {
		return err
	}

	var manifest cgmanifest
	err = json.Unmarshal(cgmanifestBytes, &manifest)
	if err != nil {
		return err
	}

	var downloadUri string
	for _, component := range manifest.Registrations {
		if component.Component.Other.Name == specName {
			downloadUri = component.Component.Other.DownloadUrl
			break
		}
	}

	if downloadUri == "" {
		return fmt.Errorf("component not found in cgmanifest: %s", specName)
	}

	slog.Info("Found download URI", "component", specName, "uri", downloadUri)

	destFilename := path.Base(downloadUri)

	return downloadFile(downloadUri, path.Join(outputDir, destFilename))
}

func downloadFile(uri string, destPath string) error {
	slog.Info("Downloading file", "uri", uri, "dest", destPath)

	// Create the file
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}

	defer out.Close()

	// Get the data
	resp, err := http.Get(uri)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	slog.Info("Download complete", "size", resp.ContentLength)

	return nil
}

func init() {
	downloadCmd.AddCommand(downloadSourcesCmd)

	downloadSourcesCmd.Flags().StringVarP(&specPath, "spec", "s", "", "spec file path")
	downloadSourcesCmd.MarkFlagRequired("spec")
	downloadSourcesCmd.MarkFlagFilename("spec")

	downloadSourcesCmd.Flags().StringVarP(&outputDir, "output-dir", "o", "", "output directory")
	downloadSourcesCmd.MarkFlagRequired("output-dir")
	downloadSourcesCmd.MarkFlagDirname("output-dir")
}
