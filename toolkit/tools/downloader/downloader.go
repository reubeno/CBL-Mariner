// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package main

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"os"
	"time"

	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/artifactcache"
	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/downloadcache"
	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/exe"
	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/logger"
	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/network"
	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/retry"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	app = kingpin.New("downloader", "Cache-aware downloader.")

	logFile  = exe.LogFileFlag(app)
	logLevel = exe.LogLevelFlag(app)

	outFile  = app.Flag("output", "Output file path.").Required().String()
	uri      = app.Flag("uri", "URI of file to download.").Required().String()
	cacheDir = app.Flag("cache", "Path to artifact cache.").String()

	caCertFile    = app.Flag("ca-cert", "Root certificate authority to use when downloading files.").String()
	tlsClientCert = app.Flag("tls-cert", "TLS client certificate to use when downloading files.").String()
	tlsClientKey  = app.Flag("tls-key", "TLS client key to use when downloading files.").String()
)

func main() {
	app.Version(exe.ToolkitVersion)
	kingpin.MustParse(app.Parse(os.Args[1:]))
	logger.InitBestEffort(*logFile, *logLevel)

	// Open the download cache if specified
	var downloadCache *downloadcache.DownloadCache
	if *cacheDir != "" {
		artifactCache, err := artifactcache.Open(*cacheDir)
		if err != nil {
			logger.PanicOnError(err)
		}

		downloadCache, err = downloadcache.Open(artifactCache)
		if err != nil {
			logger.PanicOnError(err)
		}
	}

	// Load up certs.
	caCerts, err := x509.SystemCertPool()
	logger.PanicOnError(err, "Received error calling x509.SystemCertPool(). Error: %v", err)
	if *caCertFile != "" {
		newCACert, err := ioutil.ReadFile(*caCertFile)
		if err != nil {
			logger.Log.Panicf("Invalid CA certificate (%s), error: %s", *caCertFile, err)
		}

		caCerts.AppendCertsFromPEM(newCACert)
	}

	var tlsCerts []tls.Certificate
	if *tlsClientCert != "" && *tlsClientKey != "" {
		cert, err := tls.LoadX509KeyPair(*tlsClientCert, *tlsClientKey)
		if err != nil {
			logger.Log.Panicf("Invalid TLS client key pair (%s) (%s), error: %s", *tlsClientCert, *tlsClientKey, err)
		}

		tlsCerts = append(tlsCerts, cert)
	}

	downloadFile(*uri, *outFile, downloadCache, caCerts, tlsCerts)
}

func downloadFile(uri, outputFilePath string, cache *downloadcache.DownloadCache, caCerts *x509.CertPool, tlsCerts []tls.Certificate) (err error) {
	const (
		// With 5 attempts, initial delay of 1 second, and a backoff factor of 2.0 the total time spent retrying will be
		// ~30 seconds.
		downloadRetryAttempts = 5
		failureBackoffBase    = 2.0
		downloadRetryDuration = time.Second
	)
	var noCancel chan struct{} = nil

	_, err = retry.RunWithExpBackoff(func() error {
		return network.CacheAwareDownloadFile(uri, outputFilePath, cache, caCerts, tlsCerts)
	}, downloadRetryAttempts, downloadRetryDuration, failureBackoffBase, noCancel)
	return
}
