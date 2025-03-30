package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/pddg/photon-container/internal/logging"
	"github.com/pddg/photon-container/internal/photondata"
	"github.com/pddg/photon-container/internal/unarchiver"
	"github.com/pddg/photon-container/internal/updater"
)

type Migrator interface {
	State(ctx context.Context) (photondata.MigrationState, time.Time)
	ResetState(ctx context.Context)
}

type MigrateStatusHandler struct {
	migrator Migrator
	mux      *http.ServeMux
}

// NewMigrateStatusHandler creates a new MigrateStatusHandler.
func NewMigrateStatusHandler(migrator Migrator) *MigrateStatusHandler {
	h := &MigrateStatusHandler{
		migrator: migrator,
		mux:      http.NewServeMux(),
	}
	h.mux.HandleFunc("GET /", h.get)
	h.mux.HandleFunc("DELETE /", h.delete)
	return h
}

func (h *MigrateStatusHandler) get(w http.ResponseWriter, r *http.Request) {
	state, version := h.migrator.State(r.Context())
	res := struct {
		State   string `json:"state"`
		Version string `json:"version"`
	}{
		State:   string(state),
		Version: version.Format(time.RFC3339),
	}
	resultBytes, err := json.Marshal(res)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(resultBytes)
}

func (h *MigrateStatusHandler) delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logging.FromContext(ctx).InfoContext(ctx, "Reset migration state")
	h.migrator.ResetState(ctx)
	w.WriteHeader(http.StatusOK)
}

func (h *MigrateStatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

type LocalMigrateHandler struct {
	ctx      context.Context
	migrator Migrator
	updater  updater.UpdaterInterface
	archive  photondata.Archive
}

func NewLocalMigrateHandler(
	ctx context.Context,
	migrator Migrator,
	updater updater.UpdaterInterface,
	archive photondata.Archive,
) *LocalMigrateHandler {
	return &LocalMigrateHandler{
		ctx:      ctx,
		migrator: migrator,
		updater:  updater,
		archive:  archive,
	}
}

func (h *LocalMigrateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var options []updater.UpdateOption
	userSpecified := r.URL.Query().Get("archive")
	if userSpecified != "" {
		options = append(options, updater.WithArchiveName(userSpecified))
	}
	forceMigrate := r.URL.Query().Get("force") == "true"
	if forceMigrate {
		options = append(options, updater.WithForceUpdate())
	}
	go func() {
		// Do not use r.Context() here. It may be canceled before the update is finished.
		if err := h.updater.DownloadAndUpdate(h.ctx, h.archive, options...); err != nil {
			logging.FromContext(h.ctx).ErrorContext(h.ctx, "failed to update", "error", err)
		}
	}()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("migration started. Check logs if you want to know the progress"))
}

type MigrateHandler struct {
	ctx     context.Context
	updater updater.UpdaterInterface
}

func NewMigrateHandler(ctx context.Context, updater updater.UpdaterInterface) *MigrateHandler {
	return &MigrateHandler{
		ctx:     ctx,
		updater: updater,
	}
}

func (h *MigrateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var options []updater.UpdateOption
	if r.URL.Query().Get("force") == "true" {
		options = append(options, updater.WithForceUpdate())
	}
	if r.URL.Query().Get("no_compression") == "true" {
		options = append(options, updater.WithUnarchiveOptions(
			unarchiver.NoCompression(),
		))
	}
	if err := h.updater.UpdateAsync(h.ctx, r.Body, options...); err != nil {
		logging.FromContext(h.ctx).ErrorContext(h.ctx, "failed to update", "error", err)
		// Stop unnecessary request body reading.
		r.Body.Close()
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("migration started. Check logs if you want to know the progress"))
}
