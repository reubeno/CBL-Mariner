// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package network

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/downloadcache"
	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/file"
	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/logger"
	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/retry"
	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/shell"
)

// JoinURL concatenates baseURL with extraPaths
func JoinURL(baseURL string, extraPaths ...string) string {
	const urlPathSeparator = "/"

	if len(extraPaths) == 0 {
		return baseURL
	}

	appendToBase := strings.Join(extraPaths, urlPathSeparator)
	return fmt.Sprintf("%s%s%s", baseURL, urlPathSeparator, appendToBase)
}

func CacheAwareDownloadFile(url, dst string, cache *downloadcache.DownloadCache, caCerts *x509.CertPool, tlsCerts []tls.Certificate) (err error) {
	// Make sure the output file's dir tree exists.
	os.MkdirAll(filepath.Dir(dst), os.ModePerm)

	// First see if there's a cache hit.
	if cache != nil {
		var cacheEntry *downloadcache.DownloadCacheEntry
		cacheEntry, err = cache.LookupDownloadByUri(url)
		if err != nil {
			logger.Log.Warnf("Failed to lookup download cache entry for (%s).\n%s", url, err)
			err = nil
		}

		if cacheEntry != nil {
			err = file.Copy(cacheEntry.Path, dst)
			if err == nil {
				return
			} else {
				logger.Log.Warnf("Failed to copy cached download (%s) to (%s).\n%s", cacheEntry.Path, dst, err)
				err = nil
			}
		}
	}

	// If we got down here, then it was a cache miss or no cache was present; perform the download.
	err = DownloadFile(url, dst, caCerts, tlsCerts)
	if err != nil {
		logger.Log.Warnf("Attempt to download (%s) failed. Error: %s", url, err)
		return
	}

	// If we are using a cache, cache it!
	if cache != nil {
		_, err = cache.CacheDownload(url, dst)
		if err != nil {
			logger.Log.Warnf("Failed to cache download (%s).\n%s", url, err)
			err = nil
		}
	}

	return
}

// DownloadFile downloads `url` into `dst`. `caCerts` may be nil. If there is an error `dst` will be removed.
func DownloadFile(url, dst string, caCerts *x509.CertPool, tlsCerts []tls.Certificate) (err error) {
	logger.Log.Debugf("Downloading (%s) -> (%s)", url, dst)

	dstFile, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		// If there was an error, ensure that the file is removed
		if err != nil {
			cleanupErr := file.RemoveFileIfExists(dst)
			if cleanupErr != nil {
				logger.Log.Errorf("Failed to remove failed network download file '%s': %s", dst, err)
			}
		}
		dstFile.Close()
	}()

	tlsConfig := &tls.Config{
		RootCAs:      caCerts,
		Certificates: tlsCerts,
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	client := &http.Client{
		Transport: transport,
	}

	response, err := client.Get(url)
	if err != nil {
		return
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid response: %v", response.StatusCode)
	}

	_, err = io.Copy(dstFile, response.Body)

	return
}

// CheckNetworkAccess checks whether the installer environment has network access
// This function is only executed within the ISO installation environment for kickstart-like unattended installation
func CheckNetworkAccess() (err error, hasNetworkAccess bool) {
	const (
		retryAttempts = 10
		retryDuration = time.Second
		squashErrors  = false
		activeStatus  = "active"
	)

	err = retry.Run(func() error {
		err := shell.ExecuteLive(squashErrors, "systemctl", "restart", "systemd-networkd-wait-online")
		if err != nil {
			logger.Log.Errorf("Cannot start systemd-networkd-wait-online.service")
			return err
		}

		stdout, stderr, err := shell.Execute("systemctl", "is-active", "systemd-networkd-wait-online")
		if err != nil {
			logger.Log.Errorf("Failed to query status of systemd-networkd-wait-online: %v", stderr)
			return err
		}

		serviceStatus := strings.TrimSpace(stdout)
		hasNetworkAccess = serviceStatus == activeStatus
		if !hasNetworkAccess {
			logger.Log.Warnf("No network access yet")
		}

		return err
	}, retryAttempts, retryDuration)

	if err != nil {
		logger.Log.Errorf("Failure in multiple attempts to check network access")
	}

	return
}
