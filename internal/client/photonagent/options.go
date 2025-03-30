package photonagent

import (
	"net/url"
	"time"
)

type uploadOptions struct {
	noComplession    bool
	forceUpdate      bool
	progressInterval time.Duration
}

func initUploadOptions(opts ...UploadOption) *uploadOptions {
	o := &uploadOptions{
		progressInterval: 1 * time.Minute,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func (uo *uploadOptions) toQuery() url.Values {
	v := url.Values{}
	if uo.noComplession {
		v.Set("no_compression", "true")
	}
	if uo.forceUpdate {
		v.Set("force", "true")
	}
	return v
}

type UploadOption func(*uploadOptions)

// WithNoCompressedArchive reperesents an option that the archive is not compressed.
// Server will skip decompression for the archive.
func WithNoCompressedArchive() UploadOption {
	return func(o *uploadOptions) {
		o.noComplession = true
	}
}

// WithForceUpload reperesents an option that the update process is forced.
// This will reset the migration state.
func WithForceUpload() UploadOption {
	return func(o *uploadOptions) {
		o.forceUpdate = true
	}
}

// WithProgressInterval sets the interval at which the progress of the upload is logged.
// Minimum interval is 5 second since too small interval may cause performance issues.
// The default is 1 minute.
func WithProgressInterval(interval time.Duration) UploadOption {
	return func(o *uploadOptions) {
		if interval < 5*time.Second {
			interval = 5 * time.Second
		}
		o.progressInterval = interval
	}
}
