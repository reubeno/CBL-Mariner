// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package artifactcache

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/file"
	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/filelock"
	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/logger"
)

const metadataFilename = "metadata.json"

type ArtifactCache struct {
	rootDir string
}

type ArtifactCacheEntry struct {
	ContentPath string
}

type ArtifactCacheEntryMetadata struct {
	Type string `json:"type"`
}

func Open(rootPath string) (*ArtifactCache, error) {
	return &ArtifactCache{rootDir: rootPath}, nil
}

func (ac *ArtifactCache) RootDir() string {
	return ac.rootDir
}

func (ac *ArtifactCache) LookupArtifact(artifactType, jsonKey string) (*ArtifactCacheEntry, error) {
	canonicalKey, err := ac.canonicalizeJsonKey(artifactType, jsonKey)
	if err != nil {
		return nil, fmt.Errorf("failed to canonicalize artifact key\n%w", err)
	}

	digest := ac.keyToSHA256Digest(canonicalKey)
	candidatePath := ac.getPathForArtifactMatchingSHA256Digest(digest)

	metadataPath := filepath.Join(candidatePath, metadataFilename)
	metadataFile, err := os.Open(metadataPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		logger.Log.Debugf("failed to open artifact cache metadata from '%s': %v", metadataPath, err)
		return nil, err
	}

	defer metadataFile.Close()

	metadataJsonText, err := ioutil.ReadAll(metadataFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read artifact cache metadata file '%s'\n%w", metadataPath, err)
	}

	metadata := &ArtifactCacheEntryMetadata{}
	err = json.Unmarshal(metadataJsonText, metadata)
	if err != nil {
		// Warn but don't fail; we just consider the cache entry invalid since it has invalid metadata.
		logger.Log.Warnf("failed to parse artifact cache metadata in '%s': %v", metadataPath, err)
		return nil, nil
	}

	// From here on out, if we encounter errors, then complain; since we found a syntactically valid
	// metadata file, then we know something is wrong. The caller can always re-compute and re-cache the
	// artifact.
	if metadata.Type == "" {
		return nil, errors.New("artifact cache metadata is missing artifact type")
	} else if metadata.Type != artifactType {
		return nil, fmt.Errorf("artifact cache metadata has type '%s' but expected '%s'", metadata.Type, artifactType)
	}

	contentPath := filepath.Join(candidatePath, "content")
	pathStat, err := os.Stat(contentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat artifact cached content dir '%s'\n%w", contentPath, err)
	} else if !pathStat.IsDir() {
		return nil, errors.New("artifact cached content is not a directory")
	}

	return &ArtifactCacheEntry{
		ContentPath: contentPath,
	}, nil
}

func (ac *ArtifactCache) CacheArtifact(artifactType, jsonKey string, artifactPath string) (*ArtifactCacheEntry, error) {
	if artifactType == "" {
		return nil, errors.New("cannot cache artifact with empty type")
	} else if jsonKey == "" {
		return nil, errors.New("cannot cache artifact with empty key")
	}

	// Make sure the input artifact exists. Also figure out if it's a file or dir.
	if _, err := os.Stat(artifactPath); err != nil {
		return nil, fmt.Errorf("failed to check if input path '%s' is a directory\n%w", artifactPath, err)
	}

	logger.Log.Debugf("caching artifact: %s('%s') => '%s'", artifactType, jsonKey, artifactPath)

	canonicalKey, err := ac.canonicalizeJsonKey(artifactType, jsonKey)
	if err != nil {
		return nil, fmt.Errorf("failed to canonicalize artifact key\n%w", err)
	}

	digest := ac.keyToSHA256Digest(canonicalKey)
	candidatePath := ac.getPathForArtifactMatchingSHA256Digest(digest)

	// Make sure the directory exists.
	err = os.MkdirAll(candidatePath, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to create artifact cache entry directory '%s'\n%w", candidatePath, err)
	}

	// Lock the directory for exclusive access.
	lock, err := filelock.NewLock(candidatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to lock artifact cache entry directory '%s'\n%w", candidatePath, err)
	}

	defer lock.Close()
	lock.LockExclusive()

	// Wipe any existing contents of the dir. It may have been a partial import. Or maybe
	// our caller knows more than we do and it really just wants to replace it.
	err = removeAllContentsOfDir(candidatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to remove existing artifact cache entry at '%s'\n%w", candidatePath, err)
	}

	// Create the content dir we need.
	contentDir := filepath.Join(candidatePath, "content")
	err = os.MkdirAll(contentDir, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to create artifact cache entry content directory '%s'\n%w", contentDir, err)
	}

	// Walk the input path and import its files.
	err = filepath.Walk(artifactPath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relativePath := strings.TrimPrefix(filePath, artifactPath)
		if relativePath == "" {
			if info.IsDir() {
				return nil
			} else {
				relativePath = filepath.Base(filePath)
			}
		}

		destPath := filepath.Join(contentDir, relativePath)

		if info.IsDir() {
			os.MkdirAll(destPath, os.ModePerm)
		} else if info.Mode().IsRegular() {
			cachedFilePath, err := ac.getOrAddFileMatching(filePath)
			if err != nil {
				return fmt.Errorf("failed to get or add file matching '%s'\n%w", filePath, err)
			}

			logger.Log.Debugf("creating hard link: '%s' => '%s'\n", destPath, cachedFilePath)

			err = os.Link(cachedFilePath, destPath)
			if err != nil {
				return fmt.Errorf("failed to create hard link '%s' => '%s'\n%w", destPath, cachedFilePath, err)
			}
		} else {
			return fmt.Errorf("unsupported file type for '%s'", filePath)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to import artifact cache entry content\n%w", err)
	}

	// Write the key as a separate text file. (It may not be safe to include in the metadata JSON.)
	keyFilePath := filepath.Join(candidatePath, "key")
	err = ioutil.WriteFile(keyFilePath, []byte(canonicalKey), os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to write cache artifact key\n%w", err)
	}

	// Write the metadata file. This must be done last; its existence and validity indicates this
	// is a valid cache entry.
	metadata := &ArtifactCacheEntryMetadata{
		Type: artifactType,
	}
	metadataJsonText, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize cache artifact metadata\n%w", err)
	}

	metadataFilePath := filepath.Join(candidatePath, metadataFilename)
	err = ioutil.WriteFile(metadataFilePath, metadataJsonText, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to write cache artifact metadata\n%w", err)
	}

	// TODO: Decide if we should perform any filesystem flushes.

	return &ArtifactCacheEntry{
		ContentPath: contentDir,
	}, nil
}

func (ac *ArtifactCache) LookupFileMatchingSHA256Digest(digest string) (string, error) {
	if len(digest) != 64 {
		return "", errors.New("invalid SHA256 digest")
	}

	filePath := ac.getPathForFileMatchingSHA256Digest(digest)

	fileInfo, err := os.Stat(filePath)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return "", nil
	} else if err != nil {
		return "", err
	} else if fileInfo.IsDir() {
		return "", errors.New("artifact cache entry is a directory")
	}

	return filePath, nil
}

type ArtifactVisitorFunc func(entry *ArtifactCacheEntry, err error) error

func (ac *ArtifactCache) VisitArtifacts(fn ArtifactVisitorFunc) error {
	artifactsDir := filepath.Join(ac.rootDir, "artifacts")

	matches, err := filepath.Glob(filepath.Join(artifactsDir, "??", "??", "????????????????????????????????????????????????????????????", metadataFilename))
	if err != nil {
		return fmt.Errorf("failed to enumerate artifact cache artifacts dir '%s'\n%w", artifactsDir, err)
	}

	for _, metadataPath := range matches {
		containingDirPath := filepath.Dir(metadataPath)
		metadataFile, err := os.Open(metadataPath)
		if errors.Is(err, os.ErrNotExist) {
			// If it doesn't exist, don't bother the caller with it.
			continue
		} else if err != nil {
			fn(nil, err)
			continue
		}

		metadataJsonText, err := ioutil.ReadAll(metadataFile)
		if err != nil {
			fn(nil, err)
			continue
		}

		metadata := &ArtifactCacheEntryMetadata{}
		err = json.Unmarshal(metadataJsonText, metadata)
		if err != nil {
			fn(nil, err)
			continue
		}

		if metadata.Type == "" {
			fn(nil, fmt.Errorf("artifact cache metadata at '%s' is missing artifact type", metadataPath))
			continue
		}

		entry := &ArtifactCacheEntry{
			ContentPath: filepath.Join(containingDirPath, "content"),
		}

		fn(entry, nil)
	}

	return nil
}

func (*ArtifactCache) Close() error {
	return nil
}

func (ac *ArtifactCache) getOrAddFileMatching(filePath string) (string, error) {
	hash, err := file.GenerateSHA256(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to compute SHA256 hash of '%s'\n%w", filePath, err)
	}

	candidatePath := ac.getPathForFileMatchingSHA256Digest(hash)

	_, err = os.Stat(candidatePath)
	if err == nil {
		return candidatePath, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("failed to check if candidate file '%s' exists\n%w", candidatePath, err)
	}

	containingDir := filepath.Dir(candidatePath)

	// If we got down here, then we need to add the file to the cache.
	err = os.MkdirAll(containingDir, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("failed to ensure artifact cache entry directory '%s' exists\n%w", containingDir, err)
	}

	// Copy the file to a temp location in the right destination dir; this lets us atomically
	// rename it when all data has been written.
	tempFile, err := os.CreateTemp(containingDir, "")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file in '%s'\n%w", containingDir, err)
	}

	defer os.Remove(tempFile.Name())

	sourceFile, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open '%s'\n%w", filePath, err)
	}

	defer sourceFile.Close()

	_, err = io.Copy(tempFile, sourceFile)
	if err != nil {
		return "", fmt.Errorf("failed to copy '%s' to '%s'\n%w", filePath, tempFile.Name(), err)
	}

	err = tempFile.Close()
	if err != nil {
		return "", fmt.Errorf("failed to close temporary file '%s'\n%w", tempFile.Name(), err)
	}

	err = os.Rename(tempFile.Name(), candidatePath)
	if err != nil {
		return "", fmt.Errorf("failed to rename '%s' to '%s'\n%w", tempFile.Name(), candidatePath, err)
	}

	return candidatePath, nil
}

func (ac *ArtifactCache) getPathForFileMatchingSHA256Digest(digest string) string {
	return filepath.Join(ac.rootDir, "files", digest[0:2], digest[2:4], digest[4:])
}

func (ac *ArtifactCache) getPathForArtifactMatchingSHA256Digest(digest string) string {
	return filepath.Join(ac.rootDir, "artifacts", digest[0:2], digest[2:4], digest[4:])
}

func (ac *ArtifactCache) canonicalizeJsonKey(artifactType, jsonKey string) (string, error) {
	typeTaggedJsonKey := fmt.Sprintf("{\"Type\":\"%s\", \"Key\":%s}", artifactType, jsonKey)
	canonicalKey, err := jsoncanonicalizer.Transform([]byte(typeTaggedJsonKey))
	if err != nil {
		return "", err
	}

	return string(canonicalKey), nil
}

func (ac *ArtifactCache) keyToSHA256Digest(key string) string {
	// Compute the SHA256 digest of the key and format it as a hex string.
	hasher := sha256.New()
	hasher.Write([]byte(key))
	return fmt.Sprintf("%x", hasher.Sum(nil))
}

// Removes all files and dirs under the given path without removing the root.
func removeAllContentsOfDir(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}

	// On error, we still want to try to remove as much as we can.
	err = nil
	for _, entry := range entries {
		currentErr := os.RemoveAll(filepath.Join(path, entry.Name()))
		if currentErr != nil && err == nil {
			err = currentErr
		}
	}

	return err
}
