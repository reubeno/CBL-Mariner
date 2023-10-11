// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package downloadcache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/artifactcache"
)

const (
	DownloadArtifactType = "download"
)

type DownloadCache struct {
	artifactCache *artifactcache.ArtifactCache
}

type DownloadCacheKey struct {
	Uri string `json:"uri"`
}

type DownloadCacheEntry struct {
	Path string
}

func Open(artifactCache *artifactcache.ArtifactCache) (*DownloadCache, error) {
	return &DownloadCache{artifactCache: artifactCache}, nil
}

func (dc *DownloadCache) LookupDownloadByUri(uri string) (*DownloadCacheEntry, error) {
	jsonKey, err := json.Marshal(DownloadCacheKey{Uri: uri})
	if err != nil {
		return nil, err
	}

	cacheEntry, err := dc.artifactCache.LookupArtifact(DownloadArtifactType, string(jsonKey))
	if err != nil {
		return nil, err
	} else if cacheEntry == nil {
		return nil, nil
	}

	contentDirEntries, err := os.ReadDir(cacheEntry.ContentPath)
	if err != nil {
		return nil, err
	}

	if len(contentDirEntries) != 1 {
		return nil, fmt.Errorf("expected exactly one file in download cache entry content directory '%s'", cacheEntry.ContentPath)
	}

	contentDirEntry := contentDirEntries[0]

	fileInfo, err := contentDirEntry.Info()
	if err != nil {
		return nil, err
	}

	if fileInfo.IsDir() {
		return nil, fmt.Errorf("expected exactly one file in download cache entry content directory '%s'", cacheEntry.ContentPath)
	}

	filePath := filepath.Join(cacheEntry.ContentPath, contentDirEntry.Name())

	return &DownloadCacheEntry{Path: filePath}, nil
}

func (dc *DownloadCache) LookupDownloadBySHA256Digest(sha256Digest string) (*DownloadCacheEntry, error) {
	filePath, err := dc.artifactCache.LookupFileMatchingSHA256Digest(sha256Digest)
	if err != nil {
		return nil, err
	} else if filePath == "" {
		return nil, nil
	}

	return &DownloadCacheEntry{Path: filePath}, nil
}

func (dc *DownloadCache) CacheDownload(uri, downloadedFile string) (*DownloadCacheEntry, error) {
	jsonKey, err := json.Marshal(DownloadCacheKey{Uri: uri})
	if err != nil {
		return nil, err
	}

	cacheEntry, err := dc.artifactCache.CacheArtifact(DownloadArtifactType, string(jsonKey), downloadedFile)
	if err != nil {
		return nil, err
	}

	cachedFilePath := filepath.Join(cacheEntry.ContentPath, filepath.Base(downloadedFile))

	return &DownloadCacheEntry{Path: cachedFilePath}, nil
}
