package metrics

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/pddg/photon-container/internal/photondata"
)

type Migrator interface {
	State(ctx context.Context) (photondata.MigrationState, time.Time)
}

type MigrateStatusMetrics struct {
	ctx      context.Context
	migrator Migrator

	// metrics
	migrateStatusInfoDesc *prometheus.Desc
	dataImportedTimeDesc  *prometheus.Desc
}

func NewMigrateStatusMetrics(
	ctx context.Context,
	migrator Migrator,
) *MigrateStatusMetrics {
	return &MigrateStatusMetrics{
		ctx:      ctx,
		migrator: migrator,
		migrateStatusInfoDesc: prometheus.NewDesc(
			"photon_migrate_status_info",
			"Migration status info",
			[]string{"state"},
			nil,
		),
		dataImportedTimeDesc: prometheus.NewDesc(
			"photon_data_imported_timestamp_seconds",
			"Timestamp of the photon imported data in seconds",
			nil,
			nil,
		),
	}
}

func (m *MigrateStatusMetrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- m.migrateStatusInfoDesc
	ch <- m.dataImportedTimeDesc
}

func (m *MigrateStatusMetrics) Collect(ch chan<- prometheus.Metric) {
	actualState, version := m.migrator.State(m.ctx)
	for _, state := range photondata.MigrationStates {
		value := 0
		if state == actualState {
			value = 1
		}
		ch <- prometheus.MustNewConstMetric(
			m.migrateStatusInfoDesc,
			prometheus.GaugeValue,
			float64(value),
			string(state),
		)
	}
	ch <- prometheus.MustNewConstMetric(
		m.dataImportedTimeDesc,
		prometheus.GaugeValue,
		float64(version.Unix()),
	)
}
