package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"github.com/Reggie-pan/go-shorts-generator/internal/config"
	"github.com/Reggie-pan/go-shorts-generator/internal/storage"
	"github.com/Reggie-pan/go-shorts-generator/internal/worker"
)

// corsMiddleware 添加 CORS 標頭
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// spaFileServer 為 SPA 應用提供文件服務，未找到的路由會返回 index.html
func spaFileServer(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, r.URL.Path)
		// 檢查文件是否存在
		if _, err := os.Stat(path); err != nil {
			// 文件不存在，檢查是否是 API 路由
			if strings.HasPrefix(r.URL.Path, "/api/") {
				// API 路由應該返回 404
				http.NotFound(w, r)
				return
			}
			// 否則返回 index.html 讓前端路由器處理
			r.URL.Path = "/"
		}
		fs.ServeHTTP(w, r)
	})
}

func NewRouter(cfg *config.Config, store *storage.Store, q *worker.Queue) http.Handler {
	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()

	h := &Handlers{Config: cfg, Store: store, Queue: q}

	// 中介軟體：添加 CORS 標頭
	api.Use(corsMiddleware)

	api.HandleFunc("/jobs", h.CreateJob).Methods("POST")
	api.HandleFunc("/jobs", h.ListJobs).Methods("GET")
	api.HandleFunc("/jobs", h.DeleteAllJobs).Methods("DELETE")
	api.HandleFunc("/jobs/{id}", h.GetJob).Methods("GET")
	api.HandleFunc("/jobs/{id}/result", h.DownloadResult).Methods("GET")
	api.HandleFunc("/jobs/{id}/cancel", h.CancelJob).Methods("POST")
	api.HandleFunc("/jobs/{id}", h.DeleteJob).Methods("DELETE")
	api.HandleFunc("/upload", h.UploadHandler).Methods("POST")
	api.HandleFunc("/tts/voices", h.ListVoices).Methods("GET")
	api.HandleFunc("/temp", h.CleanTempFiles).Methods("DELETE")
	api.HandleFunc("/presets/bgm", h.ListBGM).Methods("GET")
	api.HandleFunc("/fonts", h.ListFonts).Methods("GET")
	api.HandleFunc("/preview/subtitle", h.PreviewSubtitle).Methods("POST")
	api.HandleFunc("/swagger.json", h.Swagger).Methods("GET")

	// 靜態資源：BGM
	// 注意：這裡直接使用 http.FileServer 暴露目錄，需確保 BGM_PATH 正確
	r.PathPrefix("/assets/bgm/").Handler(http.StripPrefix("/assets/bgm/", http.FileServer(http.Dir(cfg.BgmPath))))

	// 使用 SPA 文件服務器處理前端路由
	r.PathPrefix("/").Handler(spaFileServer("/app/public"))

	r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Warn().Str("path", r.URL.Path).Msg("路徑不存在")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	})
	return r
}
