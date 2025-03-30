package downloader

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/fujiwara/shapeio"
	"github.com/hashicorp/go-retryablehttp"

	"github.com/pddg/photon-container/internal/logging"
	"github.com/pddg/photon-container/internal/photondata"
)

type Downloader struct {
	client *retryablehttp.Client

	// Options
	// The following fields are set by the DownloaderOption functions.

	// hideProgress disables the progress tracking of the downloader.
	// Use WithoutProgress option to set this value.
	// Default is false.
	hideProgress bool

	// progressInterval sets the interval at which the progress of the download is logged.
	// Use WithProgressInterval option to set this value.
	// Default is 1 minute.
	progressInterval time.Duration

	// limitDownloadPerSec sets the download speed limit in bytes per second.
	// Use WithDownloadSpeedLimit option to set this value.
	// Default is math.MaxFloat64.
	limitDownloadBytesPerSec float64

	// readBytesPerSec sets the read downloaded file speed limit in bytes per second.
	// Use WithReadSpeedLimit option to set this value.
	// Default is math.MaxFloat64.
	limitReadBytesPerSec float64
}

// New creates a new Downloader with the given http.Client and baseURL.
// The baseURL should be the base URL of the download server.
// e.g. "https://download1.graphhopper.com/public/"
// If the baseURL does not end with a "/", it will be appended.
func New(httpClient *http.Client, options ...DownloaderOption) *Downloader {
	client := retryablehttp.NewClient()
	client.HTTPClient = httpClient
	client.RetryMax = 10
	client.RetryWaitMin = 1 * time.Second
	client.RetryWaitMax = 10 * time.Second
	client.Backoff = retryablehttp.LinearJitterBackoff
	// Disable the default logger.
	client.Logger = nil
	// Show logs using the logger from the context.
	client.RequestLogHook = func(l retryablehttp.Logger, r *http.Request, i int) {
		logger := logging.FromContext(r.Context())
		logger.DebugContext(r.Context(), "downloader request", "method", r.Method, "url", r.URL.String())
	}
	client.ResponseLogHook = func(l retryablehttp.Logger, r *http.Response) {
		ctx := r.Request.Context()
		logger := logging.FromContext(ctx)
		logger.DebugContext(ctx, "downloader response", "status", r.Status, "content_length", humanize.Bytes(uint64(r.ContentLength)))
	}

	d := &Downloader{
		client:                   client,
		progressInterval:         1 * time.Minute,
		limitDownloadBytesPerSec: math.MaxFloat64,
		limitReadBytesPerSec:     math.MaxFloat64,
	}
	for _, opt := range options {
		opt(d)
	}
	return d
}

// Download downloads a file from the given path, and saves it to the destination.
// Download URL is constructed by concatenating the baseURL and dbPath.
// Eliminate the leading slash of dbPath.
// The download progress is logged at the interval set by the WithProgressInterval option.
// After the download is complete, the MD5 sum of the downloaded file is verified.
// If the MD5 sum does not match, the downloaded file will be removed and an error is returned.
func (d *Downloader) Download(ctx context.Context, archive photondata.Archive, dest string) error {
	logger := logging.FromContext(ctx)
	url := archive.URL()
	// Get the MD5 sum of the file first.
	// Download database file may require a long time, so we need to check the MD5 sum first.
	md5sum, err := d.getMD5Sum(ctx, url)
	if err != nil {
		return fmt.Errorf("downloader.Downloader.Download: failed to verify md5sum: %w", err)
	}

	// Skip downloading if the file already exists and the MD5 sum matches.
	if stat, err := os.Stat(dest); err == nil {
		if stat.IsDir() {
			return fmt.Errorf("downloader.Downloader.Download: destination %q is a directory", dest)
		}
		// Check the MD5 sum of the existing file.
		got, err := d.md5sumFile(ctx, dest)
		if err != nil {
			return fmt.Errorf("downloader.Downloader.Download: failed to calculate md5sum of existing file: %w", err)
		}
		if got == md5sum {
			logger.InfoContext(ctx, "file already exists", "file", dest, "md5sum", md5sum)
			return nil
		}
		logger.InfoContext(ctx, "file already exists but md5sum mismatch", "file", dest, "expected_md5sum", md5sum, "actual_md5sum", got)
	}

	logger.InfoContext(ctx, "start downloading", "url", url, "dest", dest, "md5sum", md5sum)
	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("downloader.Downloader.Download: failed to create request: %w", err)
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("downloader.Downloader.Download: falied to request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloader.Downloader.Download: failed to download: %s", resp.Status)
	}

	// Create a temp file in the same directory as the destination file.
	// This is required because the file may be on a different filesystem.
	// Rename is an atomic operation within the same filesystem.
	f, err := os.CreateTemp(path.Dir(dest), "tmp-*")
	if err != nil {
		return fmt.Errorf("downloader.Downloader.Download: failed to create temp file in %q: %w", path.Dir(dest), err)
	}
	defer func() {
		// Close the file before removing it. All errors are ignored.
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()
	// Limit the download speed.
	body := shapeio.NewReaderWithContext(resp.Body, ctx)
	body.SetRateLimit(d.limitDownloadBytesPerSec)

	var r io.Reader = body
	if !d.hideProgress {
		progress := NewProgress(ctx, resp.ContentLength, d.progressInterval, logger)
		defer progress.Stop()
		r = io.TeeReader(body, progress)
	}

	if _, err := io.Copy(f, r); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("downloader.Downloader.Download: failed to write to temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("downloader.Downloader.Download: failed to close temp file: %w", err)
	}
	logger.InfoContext(ctx, "downloaded", "url", url, "dest", dest, "size", humanize.Bytes(uint64(resp.ContentLength)))

	// Verify the MD5 sum of the downloaded file.
	// This must be done after the file is closed.
	logger.InfoContext(ctx, "verifying md5sum", "file", f.Name(), "expected_md5sum", md5sum)
	got, err := d.md5sumFile(ctx, f.Name())
	if err != nil {
		return fmt.Errorf("downloader.Downloader.Download: failed to calculate md5sum: %w", err)
	}
	if got != md5sum {
		return fmt.Errorf("downloader.Downloader.Download: md5sum mismatch: got %q, want %q", got, md5sum)
	}
	logger.InfoContext(ctx, "md5sum verified", "file", dest, "expected_md5sum", md5sum, "actual_md5sum", got)

	if err := os.Rename(f.Name(), dest); err != nil {
		return fmt.Errorf("downloader.Downloader.Download: failed to rename temp file to %q: %w", dest, err)
	}
	logger.InfoContext(ctx, "download complete", "url", url, "dest", dest)
	return nil
}

func (d *Downloader) getMD5Sum(ctx context.Context, archiveUrl string) (string, error) {
	logger := logging.FromContext(ctx)
	logger.InfoContext(ctx, "verifying m5sum of downloaded file")
	url := archiveUrl + ".md5"
	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request for md5sum: %w", err)
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("falied to request md5sum: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download md5sum: %s", resp.Status)
	}

	// Body will be as follows:
	// {{md5sum}}  {{filename}}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read md5sum: %w", err)
	}

	// Split the body by space.  The first element is the md5sum.
	fields := strings.Fields(string(body))
	if len(fields) < 2 {
		return "", fmt.Errorf("invalid md5sum format: %q", string(body))
	}
	return string(fields[0]), nil
}

func (d *Downloader) md5sumFile(ctx context.Context, file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", fmt.Errorf("failed to open file %q: %w", file, err)
	}
	defer f.Close()

	// Limit the read speed.
	r := shapeio.NewReaderWithContext(f, ctx)
	r.SetRateLimit(d.limitReadBytesPerSec)

	h := md5.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", fmt.Errorf("failed to calculate md5sum: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// GetLastModified returns the Last-Modified date that obtained from header of the given dbPath.
func (d *Downloader) GetLastModified(ctx context.Context, archive photondata.Archive) (time.Time, error) {
	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodHead, archive.URL(), nil)
	if err != nil {
		return time.Time{}, fmt.Errorf("downloader.Downloader.GetLastModified: failed to create request: %w", err)
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return time.Time{}, fmt.Errorf("downloader.Downloader.GetLastModified: failed to request: %w", err)
	}
	defer resp.Body.Close()

	// Discard the body to reuse the connection.
	// HEAD request will not have a body.
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return time.Time{}, fmt.Errorf("downloader.Downloader.GetLastModified: failed to check latest: %s", resp.Status)
	}
	lastModified, err := http.ParseTime(resp.Header.Get("Last-Modified"))
	if err != nil {
		return time.Time{}, fmt.Errorf("downloader.Downloader.GetLastModified: failed to parse Last-Modified header: %w", err)
	}
	return lastModified, nil
}
