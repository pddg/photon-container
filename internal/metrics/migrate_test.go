package metrics_test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/pddg/photon-container/internal/metrics"
	"github.com/pddg/photon-container/internal/photondata"
)

type mockMigrator struct {
	state   photondata.MigrationState
	version time.Time
}

func newMockMigrator(state photondata.MigrationState, version time.Time) *mockMigrator {
	return &mockMigrator{
		state:   state,
		version: version,
	}
}

func (m *mockMigrator) State(ctx context.Context) (photondata.MigrationState, time.Time) {
	return m.state, m.version
}

func Test_MigrateStatusMetrics(t *testing.T) {
	t.Parallel()

	newExpected := func(state photondata.MigrationState, version time.Time) io.Reader {
		base := `
# HELP photon_migrate_status_info Migration status info
# TYPE photon_migrate_status_info gauge
`
		for _, s := range photondata.MigrationStates {
			value := 0
			if s == state {
				value = 1
			}
			base += fmt.Sprintf("photon_migrate_status_info{state=\"%s\"} %d\n", s, value)
		}
		base += `
# HELP photon_data_imported_timestamp_seconds Timestamp of the photon imported data in seconds
# TYPE photon_data_imported_timestamp_seconds gauge
`
		base += fmt.Sprintf("photon_data_imported_timestamp_seconds{} %d\n", version.Unix())
		t.Logf("expected:\n%s", base)
		return strings.NewReader(base)
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		// Setup
		now := time.Now()
		migrator := newMockMigrator(photondata.MigrationStateMigrated, now)
		m := metrics.NewMigrateStatusMetrics(t.Context(), migrator)

		// Exercise
		err := testutil.CollectAndCompare(m, newExpected(photondata.MigrationStateMigrated, now))

		// Verify
		require.NoError(t, err)
	})
}
