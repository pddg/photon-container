package metrics_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/pddg/photon-container/internal/metrics"
	"github.com/pddg/photon-container/internal/photondata"
)

type mockDownloader struct {
	err          error
	mutex        sync.Mutex
	lastModified time.Time
}

func newMockDownloader(
	lastModified time.Time,
	err error,
) *mockDownloader {
	return &mockDownloader{
		lastModified: lastModified,
		err:          err,
	}
}

func (m *mockDownloader) GetLastModified(ctx context.Context, archive photondata.Archive) (time.Time, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.lastModified, m.err
}

func (m *mockDownloader) SetLastModified(lastModified time.Time) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.lastModified = lastModified
}

func (m *mockDownloader) SetError(err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.err = err
}

type mockTicker struct {
	ch chan time.Time
}

func newMockTicker() *mockTicker {
	return &mockTicker{
		ch: make(chan time.Time),
	}
}

func (m *mockTicker) C() <-chan time.Time {
	return m.ch
}

func (m *mockTicker) Tick() {
	m.ch <- time.Now()
}

func Test_LatestPhotonDataMetrics(t *testing.T) {
	t.Parallel()
	const (
		baseURL = "https://example.com/"
		dbPath  = "test"
	)
	newExpected := func(lastModified time.Time) string {
		expected := fmt.Sprintf(`
# HELP photon_latest_database_last_modified_timestamp_seconds Last modified timestamp of the latest photon database in seconds
# TYPE photon_latest_database_last_modified_timestamp_seconds gauge
photon_latest_database_last_modified_timestamp_seconds{database_name="%s",database_source="%s"} %d
`, dbPath, baseURL, lastModified.Unix())
		return expected
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		// Setup
		first := time.Now().UTC().Truncate(time.Second)
		ticker := newMockTicker()
		downloader := newMockDownloader(first, nil)
		c := metrics.NewLatestPhotonDataMetrics(
			t.Context(),
			downloader,
			photondata.NewArchive(baseURL, photondata.WithArchiveName(dbPath)),
			metrics.WithTicker(ticker.C()),
		)

		// Exercise1: Collect before updating the last modified time.
		if err := testutil.CollectAndCompare(c, strings.NewReader(newExpected(first))); err != nil {
			t.Errorf("unexpected metrics output: %v", err)
		}

		// Exercise2: Update the last modified time.
		second := first.Add(time.Hour)
		downloader.SetLastModified(second)
		ticker.Tick()
		// Wait for the ticker to update the metrics.
		time.Sleep(100 * time.Millisecond)
		if err := testutil.CollectAndCompare(c, strings.NewReader(newExpected(second))); err != nil {
			t.Errorf("unexpected metrics output: %v", err)
		}
	})
	t.Run("error", func(t *testing.T) {
		t.Parallel()
		// Setup
		initial := time.Now().UTC().Truncate(time.Second)
		ticker := newMockTicker()
		downloader := newMockDownloader(initial, nil)
		c := metrics.NewLatestPhotonDataMetrics(
			t.Context(),
			downloader,
			photondata.NewArchive(baseURL, photondata.WithArchiveName(dbPath)),
			metrics.WithTicker(ticker.C()),
		)

		// Exercise
		downloader.SetError(fmt.Errorf("error"))
		ticker.Tick()
		// Wait for the ticker to update the metrics.
		time.Sleep(100 * time.Millisecond)

		// Verify
		if err := testutil.CollectAndCompare(c, strings.NewReader(newExpected(initial))); err != nil {
			t.Errorf("unexpected metrics output: %v", err)
		}
	})
}
