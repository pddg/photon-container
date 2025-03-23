package metrics

import "time"

type LatestPhotonDataMetricsOption func(*LatestPhotonDataMetrics)

// WithTicker sets the ticker used to update the last modified time.
// This is used to test the metrics.
func WithTicker(ticker <-chan time.Time) LatestPhotonDataMetricsOption {
	return func(m *LatestPhotonDataMetrics) {
		m.ticker = ticker
	}
}
