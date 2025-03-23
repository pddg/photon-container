package photondata_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pddg/photon-container/internal/photondata"
)

type mockPhotonServer struct {
	mutex      sync.Mutex
	importTime time.Time
	err        error
}

func newMockPhotonServer(importTime time.Time, err error) *mockPhotonServer {
	return &mockPhotonServer{
		importTime: importTime,
		err:        err,
	}
}

func (m *mockPhotonServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m.err != nil {
		http.Error(w, m.err.Error(), http.StatusInternalServerError)
		return
	}
	if r.Method != http.MethodGet || r.URL.Path != "/status" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	resp := struct {
		Status  string `json:"status"`
		Version string `json:"import_date"`
	}{
		Status:  "Ok",
		Version: m.importTime.Format(time.RFC3339),
	}
	respBytes, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(respBytes)
}

func (m *mockPhotonServer) Set(version time.Time, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.importTime = version
	m.err = err
}

func setupDestDir(t *testing.T) (string, string) {
	t.Helper()
	destDataDir := filepath.Join(t.TempDir(), "dest", "photon_data")
	destOpenSearchDir := filepath.Join(destDataDir, "node_1")
	require.NoError(t, os.MkdirAll(destOpenSearchDir, 0755))
	destFile := filepath.Join(destOpenSearchDir, "hello.txt")
	require.NoError(t, os.WriteFile(destFile, []byte("dest"), 0644))
	return destDataDir, destFile
}

func setupSrcDir(t *testing.T) string {
	t.Helper()
	srcDir := filepath.Join(t.TempDir(), "src")
	srcOpenSearchDir := filepath.Join(srcDir, "photon_data", "node_1")
	require.NoError(t, os.MkdirAll(srcOpenSearchDir, 0755))
	srcFile := filepath.Join(srcOpenSearchDir, "hello.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("src"), 0644))
	return srcDir
}

func Test_Migrator_MigrateByReplace(t *testing.T) {
	t.Parallel()
	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		// Setup
		now := time.Now()
		mockPhoton := newMockPhotonServer(now, nil)
		srv := httptest.NewServer(mockPhoton)
		defer srv.Close()

		// Setup files to be replaced
		destDataDir, destFile := setupDestDir(t)
		// Setup source files
		srcDir := setupSrcDir(t)

		migrator := photondata.NewMigrator(destDataDir, srv.Client(), photondata.WithPhotonURL(srv.URL))

		// Exercise
		err := migrator.MigrateByReplace(t.Context(), srcDir)

		// Verify
		require.NoError(t, err)
		got, err := os.ReadFile(destFile)
		require.NoError(t, err)
		assert.Equal(t, "src", string(got))
	})
	t.Run("recover when failed to rename", func(t *testing.T) {
		t.Parallel()
		// Setup
		now := time.Now()
		mockPhoton := newMockPhotonServer(now, nil)
		srv := httptest.NewServer(mockPhoton)
		defer srv.Close()

		// Setup files to be replaced
		destDataDir, destFile := setupDestDir(t)
		// Migrate will fail because the source directory is empty
		srcDir := filepath.Join(t.TempDir(), "src")
		require.NoError(t, os.MkdirAll(srcDir, 0755))

		migrator := photondata.NewMigrator(destDataDir, srv.Client(), photondata.WithPhotonURL(srv.URL))

		// Exercise
		err := migrator.MigrateByReplace(t.Context(), srcDir)

		// Verify
		require.Error(t, err)
		got, err := os.ReadFile(destFile)
		require.NoError(t, err)
		// The file should not be replaced
		assert.Equal(t, "dest", string(got))
	})
	t.Run("migration blocked when it is in progress", func(t *testing.T) {
		t.Parallel()
		// Setup
		now := time.Now()
		mockPhoton := newMockPhotonServer(now, nil)
		srv := httptest.NewServer(mockPhoton)
		defer srv.Close()
		migrator := photondata.NewMigrator(t.TempDir(), srv.Client(), photondata.WithPhotonURL(srv.URL))

		// Exercise
		// First migration. It should NOT be succeeded.
		err := migrator.MigrateByReplace(t.Context(), t.TempDir())
		require.Error(t, err)
		// Second migration. It should be blocked.
		err = migrator.MigrateByReplace(t.Context(), t.TempDir())

		// Verify
		require.ErrorIs(t, err, photondata.ErrMigrationInProgress)
	})
}

func Test_Migrator_MigrateByRemoveFirst(t *testing.T) {
	t.Parallel()
	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		// Setup
		now := time.Now()
		mockPhoton := newMockPhotonServer(now, nil)
		srv := httptest.NewServer(mockPhoton)
		defer srv.Close()

		// Setup files to be removed
		destDataDir, destFile := setupDestDir(t)
		// Setup source files
		srcDir := setupSrcDir(t)

		migrator := photondata.NewMigrator(destDataDir, srv.Client(), photondata.WithPhotonURL(srv.URL))

		// Exercise1: Remove the destination directory
		runMigration, err := migrator.MigrateByRemoveFirst(t.Context(), srcDir)
		require.NoError(t, err)

		// Verify1: Destination directory should be removed
		_, err = os.Stat(filepath.Join(destDataDir, "node_1"))
		require.ErrorIs(t, err, os.ErrNotExist)

		// Exercise2: Run the migration
		err = runMigration()

		// Verify2: The source file should be copied to the destination
		require.NoError(t, err)
		got, err := os.ReadFile(destFile)
		require.NoError(t, err)
		assert.Equal(t, "src", string(got))
	})
	t.Run("migration blocked when it is in progress", func(t *testing.T) {
		t.Parallel()
		// Setup
		now := time.Now()
		mockPhoton := newMockPhotonServer(now, nil)
		srv := httptest.NewServer(mockPhoton)
		defer srv.Close()
		migrator := photondata.NewMigrator(t.TempDir(), srv.Client(), photondata.WithPhotonURL(srv.URL))

		// Exercise
		// First migration. It should succeed.
		_, err := migrator.MigrateByRemoveFirst(t.Context(), t.TempDir())
		require.NoError(t, err)
		// Do not call actual migration function.
		// Second migration. It should be blocked.
		_, err = migrator.MigrateByRemoveFirst(t.Context(), t.TempDir())

		// Verify
		require.ErrorIs(t, err, photondata.ErrMigrationInProgress)
	})
}

func Test_Migrator_State(t *testing.T) {
	t.Parallel()
	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		// Setup
		now := time.Now().UTC().Truncate(time.Second)
		mockPhoton := newMockPhotonServer(now, nil)
		srv := httptest.NewServer(mockPhoton)
		defer srv.Close()

		migrator := photondata.NewMigrator(t.TempDir(), srv.Client(), photondata.WithPhotonURL(srv.URL))

		// Exercise1: Photon server responds with the import date
		state, version := migrator.State(t.Context())

		// Verify1
		assert.Equal(t, photondata.MigrationStateMigrated, state)
		assert.Equal(t, now, version)

		// Exercise2: Photon server returns error
		mockPhoton.Set(time.Time{}, assert.AnError)
		state, version = migrator.State(t.Context())

		// Verify2: The previous state and version should be returned
		assert.Equal(t, photondata.MigrationStateMigrated, state)
		assert.Equal(t, now, version)
	})
	t.Run("error", func(t *testing.T) {
		t.Parallel()
		// Setup
		mockPhoton := newMockPhotonServer(time.Time{}, assert.AnError)
		srv := httptest.NewServer(mockPhoton)
		defer srv.Close()
		migrator := photondata.NewMigrator(t.TempDir(), srv.Client(), photondata.WithPhotonURL(srv.URL))

		// Exercise
		state, version := migrator.State(t.Context())

		// Verify
		assert.Equal(t, photondata.MigrationStateUnknown, state)
		assert.Equal(t, time.Time{}, version)
	})
}

func Test_Migrator_ResetState(t *testing.T) {
	t.Parallel()
	t.Run("photon server works", func(t *testing.T) {
		t.Parallel()
		// Setup
		now := time.Now().UTC().Truncate(time.Second)
		mockPhoton := newMockPhotonServer(now, nil)
		srv := httptest.NewServer(mockPhoton)
		defer srv.Close()

		migrator := photondata.NewMigrator(t.TempDir(), srv.Client(), photondata.WithPhotonURL(srv.URL))
		// Fail the migration to set the state to migrating
		err := migrator.MigrateByReplace(t.Context(), t.TempDir())
		require.Error(t, err)

		// Exercise: Reset the state
		migrator.ResetState(t.Context())

		// Verify: Get the state
		state, version := migrator.State(t.Context())
		assert.Equal(t, photondata.MigrationStateMigrated, state)
		assert.Equal(t, now, version)
	})
	t.Run("photon server does not work", func(t *testing.T) {
		t.Parallel()
		// Setup
		mockPhoton := newMockPhotonServer(time.Time{}, assert.AnError)
		srv := httptest.NewServer(mockPhoton)
		defer srv.Close()

		migrator := photondata.NewMigrator(t.TempDir(), srv.Client(), photondata.WithPhotonURL(srv.URL))
		// Fail the migration to set the state to migrating
		err := migrator.MigrateByReplace(t.Context(), t.TempDir())
		require.Error(t, err)

		// Exercise: Reset the state
		migrator.ResetState(t.Context())

		// Verify: Get the state
		state, version := migrator.State(t.Context())
		assert.Equal(t, photondata.MigrationStateUnknown, state)
		assert.Equal(t, time.Time{}, version)
	})
}
