package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/pddg/photon-container/internal/logging"
	"github.com/pddg/photon-container/internal/photondata"
)

// LatestPhotonDataMetrics is a prometheus.Collector that collects the last modified timestamp of the latest photon database.
// It uses the Downloader to get the last modified timestamp.
// The last modified timestamp is updated every hour by default.
type LatestPhotonDataMetrics struct {
	// ctx is the context used for the downloader.  // Save context to the struct is anti-pattern.
	// But the prometheus library does not provide a way to pass the context when collecting metrics.
	// So, we have to save the context to the struct.
	ctx              context.Context
	archive          photondata.Archive
	downloader       Downloader
	lastModifiedDesc *prometheus.Desc

	mutex            sync.Mutex
	lastModifiedTime time.Time

	// Optional fields

	// ticker is the ticker used to update the last modified time.
	// It is set to 1 hour by default.
	ticker <-chan time.Time
}

type Downloader interface {
	GetLastModified(ctx context.Context, archive photondata.Archive) (time.Time, error)
}

func NewLatestPhotonDataMetrics(
	ctx context.Context,
	downloader Downloader,
	archive photondata.Archive,
	options ...LatestPhotonDataMetricsOption,
) *LatestPhotonDataMetrics {
	ticker := time.NewTicker(1 * time.Hour)
	pm := &LatestPhotonDataMetrics{
		ctx:        ctx,
		archive:    archive,
		downloader: downloader,
		ticker:     ticker.C,
		lastModifiedDesc: prometheus.NewDesc(
			"photon_latest_database_last_modified_timestamp_seconds",
			"Last modified timestamp of the latest photon database in seconds",
			[]string{"database_source", "database_name"},
			nil,
		),
	}
	for _, option := range options {
		option(pm)
	}
	// Update the last modified time immediately.
	pm.updateLastModifiedTime()
	go func() {
		// Close the ticker when exiting the function.
		// This should be done to avoid leak if the ticker is replaced by option.
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-pm.ticker:
				pm.updateLastModifiedTime()
			}
		}
	}()
	return pm
}

func (pm *LatestPhotonDataMetrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- pm.lastModifiedDesc
}

func (pm *LatestPhotonDataMetrics) Collect(ch chan<- prometheus.Metric) {
	pm.mutex.Lock()
	lastModified := float64(pm.lastModifiedTime.Unix())
	pm.mutex.Unlock()
	ch <- prometheus.MustNewConstMetric(
		pm.lastModifiedDesc,
		prometheus.GaugeValue,
		lastModified,
		pm.archive.BaseURL(),
		pm.archive.Name(),
	)
}

func (pm *LatestPhotonDataMetrics) updateLastModifiedTime() {
	logger := logging.FromContext(pm.ctx)
	lastModified, err := pm.downloader.GetLastModified(pm.ctx, pm.archive)
	if err != nil {
		logger.WarnContext(pm.ctx, "failed to get last modified timestamp", "error", err)
		return
	}
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	pm.lastModifiedTime = lastModified
}
