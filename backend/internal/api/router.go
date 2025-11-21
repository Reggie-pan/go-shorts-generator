package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"video-smith/backend/internal/config"
	"video-smith/backend/internal/storage"
	"video-smith/backend/internal/worker"
)

func NewRouter(cfg *config.Config, store *storage.Store, q *worker.Queue) http.Handler {
	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()

	h := &Handlers{Config: cfg, Store: store, Queue: q}

	api.HandleFunc("/jobs", h.CreateJob).Methods("POST")
	api.HandleFunc("/jobs", h.ListJobs).Methods("GET")
	api.HandleFunc("/jobs/{id}", h.GetJob).Methods("GET")
	api.HandleFunc("/jobs/{id}/result", h.DownloadResult).Methods("GET")
	api.HandleFunc("/jobs/{id}/cancel", h.CancelJob).Methods("POST")
	api.HandleFunc("/jobs/{id}", h.DeleteJob).Methods("DELETE")
	api.HandleFunc("/presets/bgm", h.ListBGM).Methods("GET")
	api.HandleFunc("/swagger.json", h.Swagger).Methods("GET")

	fs := http.FileServer(http.Dir("/app/public"))
	r.PathPrefix("/").Handler(fs)

	r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Warn().Str("path", r.URL.Path).Msg("路徑不存在")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	})
	return r
}
