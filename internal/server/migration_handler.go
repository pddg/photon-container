package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/pddg/photon-container/internal/logging"
	"github.com/pddg/photon-container/internal/photondata"
)

type Migrator interface {
	State(ctx context.Context) (photondata.MigrationState, time.Time)
	ResetState(ctx context.Context)
}

type Updater interface {
	UpdateByLocalArchive(ctx context.Context, archive photondata.Archive, force bool) error
	UpdateAsync(ctx context.Context, archive io.Reader) error
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
	h.migrator.ResetState(r.Context())
	w.WriteHeader(http.StatusOK)
}

func (h *MigrateStatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

type LocalMigrateHandler struct {
	ctx      context.Context
	migrator Migrator
	updater  Updater
	archive  photondata.Archive
}

func NewLocalMigrateHandler(
	ctx context.Context,
	migrator Migrator,
	updater Updater,
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
	var archive photondata.Archive
	latest := r.URL.Query().Get("latest") == "true"
	if latest {
		archive = h.archive
	} else {
		userSpecified := r.URL.Query().Get("archive")
		if userSpecified == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("archive query parameter is required when latest is not set"))
			return
		}
		archive = h.archive.FromArchiveName(userSpecified)
	}
	forceMigrate := r.URL.Query().Get("force") == "true"
	if forceMigrate {
		logging.FromContext(h.ctx).WarnContext(h.ctx, "force migration initiated")
		h.migrator.ResetState(r.Context())
	}
	go func() {
		// Do not use r.Context() here. It may be canceled before the update is finished.
		if err := h.updater.UpdateByLocalArchive(h.ctx, archive, forceMigrate); err != nil {
			logging.FromContext(h.ctx).ErrorContext(h.ctx, "failed to update", "error", err)
		}
	}()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("migration started. Check logs if you want to know the progress"))
}

type MigrateHandler struct {
	ctx     context.Context
	updater Updater
}

func NewMigrateHandler(ctx context.Context, updater Updater) *MigrateHandler {
	return &MigrateHandler{
		ctx:     ctx,
		updater: updater,
	}
}

func (h *MigrateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h.updater.UpdateAsync(h.ctx, r.Body); err != nil {
		logging.FromContext(h.ctx).ErrorContext(h.ctx, "failed to update", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("migration started. Check logs if you want to know the progress"))
}
