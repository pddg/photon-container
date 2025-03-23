package downloader

import "time"

const (
	DefaultDatabaseURL = "https://download1.graphhopper.com/public/experimental/"
)

type DownloaderOption func(*Downloader)

// WithoutProgress disables the progress tracking of the downloader.
func WithoutProgress() DownloaderOption {
	return func(d *Downloader) {
		d.hideProgress = true
	}
}

// WithProgressInterval sets the interval at which the progress of the download is logged.
// The default is 1 minute.
func WithProgressInterval(interval time.Duration) DownloaderOption {
	return func(d *Downloader) {
		d.progressInterval = interval
	}
}

// WithDownloadSpeedLimit sets the download speed limit in bytes per second.
// The default is math.MaxFloat64.
func WithDownloadSpeedLimit(limit float64) DownloaderOption {
	return func(d *Downloader) {
		d.limitDownloadBytesPerSec = limit
	}
}

// WithReadSpeedLimit sets the read downloaded file speed limit in bytes per second.
// The default is math.MaxFloat64.
func WithReadSpeedLimit(limit float64) DownloaderOption {
	return func(d *Downloader) {
		d.limitReadBytesPerSec = limit
	}
}
