package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/hashicorp/go-cleanhttp"

	"github.com/pddg/photon-container/internal/client/photonwrapper"
	"github.com/pddg/photon-container/internal/downloader"
	"github.com/pddg/photon-container/internal/logging"
	"github.com/pddg/photon-container/internal/photondata"
)

var (
	logLevel                      string
	logFormat                     string
	databaseURL                   string
	databaseCountryCode           string
	archivePath                   string
	archiveDownloadPath           string
	downloadOnly                  bool
	waitUntilDone                 bool
	noComplessed                  bool
	force                         bool
	photonWrapperURL              string
	progressIntervalStr           string
	downloadSpeedLimitBytesPerSec string
)

func main() {
	flag.StringVar(&logLevel, "log-level", getEnv("PHOTON_WRAPPER_LOG_LEVEL", "info"), "log level")
	flag.StringVar(&logFormat, "log-format", getEnv("PHOTON_WRAPPER_LOG_FORMAT", "json"), "log format")

	flag.StringVar(&databaseURL, "database-url", getEnv("PHOTON_WRAPPER_DATABASE_URL", downloader.DefaultDatabaseURL), "URL of the Photon database to download")
	flag.StringVar(&databaseCountryCode, "database-country-code", getEnv("PHOTON_WRAPPER_DATABASE_COUNTRY_CODE", ""), "country code of the Photon database to download. If empty, download the full database")

	flag.StringVar(&archivePath, "archive", getEnv("PHOTON_UPDATER_ARCHIVE", ""), "path to the local archive if you want to use it instead of downloading")
	flag.StringVar(&archiveDownloadPath, "download-to", getEnv("PHOTON_UPDATER_DOWNLOAD_TO", "/tmp/photon-db.tar.bz2"), "path to download the archive. Skip downloading if md5sum matches with the existing file")
	flag.StringVar(&photonWrapperURL, "photon-wrapper-url", getEnv("PHOTON_WRAPPER_URL", "http://localhost:8080"), "URL of the photon-wrapper server")
	flag.BoolVar(&downloadOnly, "download-only", getEnv("PHOTON_UPDATER_DOWNLOAD_ONLY", "false") == "true", "only download the archive and exit")
	flag.BoolVar(&waitUntilDone, "wait", getEnv("PHOTON_UPDATER_WAIT", "false") == "true", "wait until the migration is done")
	flag.BoolVar(&noComplessed, "no-compressed", getEnv("PHOTON_UPDATER_NO_COMPRESSED", "false") == "true", "Archive is not compressed. Server will skip decompression")
	flag.BoolVar(&force, "force", getEnv("PHOTON_UPDATER_FORCE", "false") == "true", "force to initiate migration")
	flag.StringVar(&progressIntervalStr, "progress-interval", getEnv("PHOTON_UPDATER_PROGRESS_INTERVAL", "1m"), "progress interval. e.g. 1m, 5s")
	flag.StringVar(&downloadSpeedLimitBytesPerSec, "download-speed-limit", getEnv("PHOTON_UPDATER_DOWNLOAD_SPEED_LIMIT", ""), "download speed limit in bytes per second (e.g. 10MB). default is unlimited")
	flag.Parse()

	logger, err := logging.Configure(logLevel, logFormat, os.Stderr)
	if err != nil {
		log.Fatalf("failed to configure logging: %v", err)
	}
	ctx := logging.NewContext(context.Background(), logger)
	if err := innerMain(ctx); err != nil {
		logger.Error("failed", "error", err)
		os.Exit(1)
	}
}

func innerMain(ctx context.Context) error {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	logger := logging.FromContext(ctx)

	httpClient := cleanhttp.DefaultClient()
	progressInterval, err := time.ParseDuration(progressIntervalStr)
	if err != nil {
		return fmt.Errorf("failed to parse progress interval: %w", err)
	}
	archiveOptions, downloadOptions, uploadOptions, err := initOptions(progressInterval)
	if err != nil {
		return err
	}
	archive := photondata.NewArchive(databaseURL, archiveOptions...)
	dl := downloader.New(httpClient, downloadOptions...)
	wrapperClient := photonwrapper.NewClient(httpClient, photonWrapperURL)

	if archivePath == "" {
		if err := dl.Download(ctx, archive, archiveDownloadPath); err != nil {
			return err
		}
		archivePath = archiveDownloadPath
	}

	if downloadOnly {
		logger.InfoContext(ctx, "download only mode is enabled. Exiting...")
		return nil
	}

	logger.InfoContext(ctx, "start uploading photon database. this may take a while", "archive", archivePath)
	if err := wrapperClient.MigrateStart(ctx, archivePath, uploadOptions...); err != nil {
		return err
	}
	if waitUntilDone {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(progressInterval):
				resp, err := wrapperClient.MigrateStatus(ctx)
				if err != nil {
					return err
				}
				if resp.State == photondata.MigrationStateMigrated {
					logger.InfoContext(ctx, "migration is done", "version", resp.Version)
					return nil
				}
				logger.InfoContext(ctx, "migration is in progress", "state", resp.State)
			}
		}
	}
	logger.InfoContext(ctx, "migration has been started. See server logs for the progress")
	return nil
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func initOptions(
	progressInterval time.Duration,
) (
	[]photondata.ArchiveOption,
	[]downloader.DownloaderOption,
	[]photonwrapper.UploadOption,
	error,
) {
	var (
		archiveOptions  []photondata.ArchiveOption
		downloadOptions []downloader.DownloaderOption
		uploadOptions   []photonwrapper.UploadOption
	)
	downloadOptions = append(downloadOptions, downloader.WithProgressInterval(progressInterval))
	uploadOptions = append(uploadOptions, photonwrapper.WithProgressInterval(progressInterval))
	if downloadSpeedLimitBytesPerSec != "" {
		limitBytes, err := humanize.ParseBytes(downloadSpeedLimitBytesPerSec)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to parse download speed limit: %w", err)
		}
		downloadOptions = append(downloadOptions, downloader.WithDownloadSpeedLimit(float64(limitBytes)))
	}
	if databaseCountryCode != "" {
		archiveOptions = append(archiveOptions, photondata.WithCountryCode(databaseCountryCode))
	}
	if force {
		uploadOptions = append(uploadOptions, photonwrapper.WithForceUpload())
	}
	if noComplessed {
		uploadOptions = append(uploadOptions, photonwrapper.WithNoCompressedArchive())
	}
	return archiveOptions, downloadOptions, uploadOptions, nil
}
