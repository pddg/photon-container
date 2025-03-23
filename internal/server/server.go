package server

import (
	"context"
	"net/http"

	"github.com/pddg/photon-container/internal/photondata"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type APIServer struct {
	mux *http.ServeMux
}

func NewAPIServer(
	ctx context.Context,
	migrator Migrator,
	updater Updater,
	archiveName photondata.Archive,
) *APIServer {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.Handle("/migrate/status", NewMigrateStatusHandler(migrator))
	mux.Handle("POST /migrate/download", NewLocalMigrateHandler(ctx, migrator, updater, archiveName))
	mux.Handle("POST /migrate/upload", NewMigrateHandler(ctx, updater))

	return &APIServer{
		mux: mux,
	}
}

func (s *APIServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
