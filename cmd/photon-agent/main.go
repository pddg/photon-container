package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/dustin/go-humanize"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/pddg/photon-container/internal/downloader"
	"github.com/pddg/photon-container/internal/logging"
	"github.com/pddg/photon-container/internal/metrics"
	"github.com/pddg/photon-container/internal/photon"
	"github.com/pddg/photon-container/internal/photondata"
	"github.com/pddg/photon-container/internal/server"
	"github.com/pddg/photon-container/internal/unarchiver"
	"github.com/pddg/photon-container/internal/updater"
)

var (
	port                          int
	logLevel                      string
	logFormat                     string
	databaseURL                   string
	databaseCountryCode           string
	updateStrategy                string
	downloadSpeedLimitBytesPerSec string
	ioSpeedLimitBytesPerSec       string
	photonJarPath                 string
	photonDir                     string
	disableMetrics                bool
)

func main() {
	flag.StringVar(&logLevel, "log-level", getEnv("PHOTON_AGENT_LOG_LEVEL", "info"), "log level")
	flag.StringVar(&logFormat, "log-format", getEnv("PHOTON_AGENT_LOG_FORMAT", "json"), "log format")
	// Photon agent server options
	flag.IntVar(&port, "port", 8080, "port to listen on")
	flag.BoolVar(&disableMetrics, "disable-metrics", false, "disable photon database metrics (/metrics only provide go runtime information)")

	// Photon database source options
	flag.StringVar(&databaseURL, "database-url", getEnv("PHOTON_AGENT_DATABASE_URL", downloader.DefaultDatabaseURL), "URL of the Photon database to download")
	flag.StringVar(&databaseCountryCode, "database-country-code", getEnv("PHOTON_AGENT_DATABASE_COUNTRY_CODE", ""), "country code of the Photon database to download. If empty, download the full database")

	// Photon server options
	flag.StringVar(&photonJarPath, "photon-jar-path", getEnv("PHOTON_AGENT_PHOTON_JAR_PATH", "/photon/photon.jar"), "path to the Photon jar file")
	flag.StringVar(&photonDir, "photon-dir", getEnv("PHOTON_AGENT_PHOTON_DIR", "/photon"), "directory to store the Photon data")
	flag.StringVar(&updateStrategy, "update-strategy", getEnv("PHOTON_AGENT_UPDATE_STRATEGY", string(updater.DefaultUpdateStrategy)), "update strategy for the Photon database")

	// Speed limit options
	flag.StringVar(&downloadSpeedLimitBytesPerSec, "download-speed-limit", getEnv("PHOTON_AGENT_DOWNLOAD_SPEED_LIMIT", ""), "download speed limit in bytes per second (e.g. 10MB). default is unlimited")
	flag.StringVar(&ioSpeedLimitBytesPerSec, "io-speed-limit", getEnv("PHOTON_AGENT_IO_SPEED_LIMIT", ""), "I/O speed limit in bytes per second (e.g. 100MB). default is unlimited")
	flag.Parse()

	logger, err := logging.Configure(logLevel, logFormat, os.Stderr)
	if err != nil {
		log.Fatalf("failed to setup logger: %v", err)
	}
	ctx := logging.NewContext(context.Background(), logger)
	if err := innerMain(ctx); err != nil {
		logger.ErrorContext(ctx, "failed", "error", err)
		os.Exit(1)
	}
}

func innerMain(ctx context.Context) error {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	logger := logging.FromContext(ctx)
	accessLogger, err := logging.Configure("info", logFormat, os.Stdout)
	if err != nil {
		return fmt.Errorf("failed to setup access logger: %w", err)
	}
	httpClient := cleanhttp.DefaultClient()
	var (
		downloaderOptions []downloader.DownloaderOption
		unarchiverOptions []unarchiver.UnarchiverOption
		archiveOptions    []photondata.ArchiveOption
	)
	if ioSpeedLimitBytesPerSec != "" {
		ioSpeedLimit, err := parseSpeedLimit(ioSpeedLimitBytesPerSec)
		if err != nil {
			return fmt.Errorf("failed to parse I/O speed limit: %w", err)
		}
		unarchiverOptions = append(unarchiverOptions, unarchiver.WithUnarchiveLimitBytesPerSec(ioSpeedLimit))
		downloaderOptions = append(downloaderOptions, downloader.WithReadSpeedLimit(ioSpeedLimit))
	}
	if downloadSpeedLimitBytesPerSec != "" {
		downloadSpeedLimit, err := parseSpeedLimit(downloadSpeedLimitBytesPerSec)
		if err != nil {
			return fmt.Errorf("failed to parse download speed limit: %w", err)
		}
		downloaderOptions = append(downloaderOptions, downloader.WithDownloadSpeedLimit(downloadSpeedLimit))
	}
	if databaseCountryCode != "" {
		archiveOptions = append(archiveOptions, photondata.WithCountryCode(databaseCountryCode))
	}

	photonArchive := photondata.NewArchive(databaseURL, archiveOptions...)
	dl := downloader.New(httpClient, downloaderOptions...)
	ua := unarchiver.NewUnarchiver(unarchiverOptions...)
	photonServer := photon.NewPhotonServer(ctx, photonJarPath, photonDir)
	photonDataDir := filepath.Join(photonDir, "photon_data")
	migrator := photondata.NewMigrator(photonDataDir, httpClient)
	updater, err := updater.New(
		updater.NewUpdateStrategy(updateStrategy),
		dl,
		ua,
		photonServer,
		migrator,
		photonDataDir,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize updater: %w", err)
	}

	if !disableMetrics {
		latestDataMetrics := metrics.NewLatestPhotonDataMetrics(ctx, dl, photonArchive)
		prometheus.MustRegister(latestDataMetrics)
		migrateMetrics := metrics.NewMigrateStatusMetrics(ctx, migrator)
		prometheus.MustRegister(migrateMetrics)
	}

	apiHandler := server.NewAPIServer(ctx, migrator, updater, photonArchive)
	accessLogMw := logging.NewAccessLogMiddleware(accessLogger)
	srv := http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: accessLogMw.Use(apiHandler),
	}
	go func() {
		<-ctx.Done()
		shutdownCtx := context.WithoutCancel(ctx)
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.ErrorContext(shutdownCtx, "failed to shutdown server", "error", err)
		}
	}()
	logger.InfoContext(ctx, "starting photon", "port", 2322)
	if err := photonServer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start Photon server: %w", err)
	}
	logger.InfoContext(ctx, "starting server", "port", port)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("failed to start server: %w", err)
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func parseSpeedLimit(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	speed, err := humanize.ParseBytes(s)
	if err != nil {
		return 0, fmt.Errorf("%w", err)
	}
	return float64(speed), nil
}
