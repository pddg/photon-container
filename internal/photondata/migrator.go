package photondata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pddg/photon-container/internal/logging"
)

var ErrMigrationInProgress = fmt.Errorf("migration in progress")

type MigrationState string

const (
	MigrationStateUnknown   MigrationState = "unknown"
	MigrationStateMigrated  MigrationState = "migrated"
	MigrationStateMigrating MigrationState = "migrating"
)

var MigrationStates = []MigrationState{
	MigrationStateUnknown,
	MigrationStateMigrated,
	MigrationStateMigrating,
}

type Migrator struct {
	dataDir    string
	httpClient *http.Client

	photonURL string

	mutex         sync.Mutex
	state         MigrationState
	cachedModTime time.Time
}

func NewMigrator(photonDataDir string, httpClient *http.Client, options ...MigratorOption) *Migrator {
	m := &Migrator{
		// OpenSearch compatible data directory is named as `node_1`.
		// Elasticsearch compatible data directory is named as `elasticsearch`.
		dataDir:    filepath.Join(photonDataDir, "node_1"),
		httpClient: httpClient,
		photonURL:  "http://localhost:2322/",
		state:      MigrationStateUnknown,
	}
	for _, option := range options {
		option(m)
	}
	return m
}

func (m *Migrator) State(ctx context.Context) (MigrationState, time.Time) {
	importTime, err := m.getVersion(ctx)
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if err != nil {
		return m.state, m.cachedModTime
	}
	if m.state == MigrationStateUnknown {
		m.state = MigrationStateMigrated
	}
	m.cachedModTime = importTime
	return m.state, importTime
}

func (m *Migrator) getVersion(ctx context.Context) (time.Time, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.photonURL+"status", nil)
	if err != nil {
		return time.Time{}, err
	}
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return time.Time{}, err
	}
	var statusJson struct {
		Status     string `json:"status"`
		ImportDate string `json:"import_date"`
	}
	if err := json.Unmarshal(bodyBytes, &statusJson); err != nil {
		return time.Time{}, err
	}
	if strings.ToLower(statusJson.Status) != "ok" {
		return time.Time{}, fmt.Errorf("status is not OK: %s %q", statusJson.Status, string(bodyBytes))
	}
	d, err := time.Parse(time.RFC3339, statusJson.ImportDate)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse import_date %q: %w", statusJson.ImportDate, err)
	}
	return d.UTC(), nil
}

func (m *Migrator) MigrateByReplace(ctx context.Context, unarchived string) error {
	logger := logging.FromContext(ctx)
	m.mutex.Lock()
	if m.state == MigrationStateMigrating {
		m.mutex.Unlock()
		return fmt.Errorf("photondata.Migrator.MigrateByReplace: %w", ErrMigrationInProgress)
	}
	m.state = MigrationStateMigrating
	m.mutex.Unlock()

	oldDir := m.dataDir + ".old"
	if err := os.Rename(m.dataDir, oldDir); err != nil {
		return fmt.Errorf("photondata.Migrator.MigrateByReplace: failed to rename %q to %q: %w", m.dataDir, oldDir, err)
	}

	unarchivedDataDir := filepath.Join(unarchived, "photon_data", "node_1")
	if err := os.Rename(unarchivedDataDir, m.dataDir); err != nil {
		if renameErr := os.Rename(oldDir, m.dataDir); renameErr != nil {
			logger.WarnContext(ctx, "failed to restore old database", "path", oldDir, "error", renameErr)
		}
		return fmt.Errorf("photondata.Migrator.MigrateByReplace: failed to rename %q to %q: %w", unarchivedDataDir, m.dataDir, err)
	}
	if err := os.RemoveAll(oldDir); err != nil {
		logger.WarnContext(ctx, "failed to remove old database", "path", oldDir, "error", err)
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.state = MigrationStateMigrated
	return nil
}

func (m *Migrator) MigrateByRemoveFirst(ctx context.Context, unarchived string) (func() error, error) {
	m.mutex.Lock()
	if m.state == MigrationStateMigrating {
		m.mutex.Unlock()
		return nil, fmt.Errorf("photondata.Migrator.MigrateByRemoveFirst: %w", ErrMigrationInProgress)
	}
	m.state = MigrationStateMigrating
	m.mutex.Unlock()

	if err := os.RemoveAll(m.dataDir); err != nil {
		return nil, fmt.Errorf("photondata.Migrator.MigrateByRemoveFirst: failed to remove %q: %w", m.dataDir, err)
	}
	return func() error {
		unarchivedDataDir := filepath.Join(unarchived, "photon_data", "node_1")
		if err := os.Rename(unarchivedDataDir, m.dataDir); err != nil {
			return fmt.Errorf("photondata.Migrator.MigrateByRemoveFirst: failed to rename %q to %q: %w", unarchivedDataDir, m.dataDir, err)
		}
		m.mutex.Lock()
		defer m.mutex.Unlock()
		m.state = MigrationStateMigrated
		return nil
	}, nil
}

func (m *Migrator) ResetState(ctx context.Context) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if _, err := m.getVersion(ctx); err != nil {
		m.state = MigrationStateUnknown
		return
	}
	m.state = MigrationStateMigrated
}
