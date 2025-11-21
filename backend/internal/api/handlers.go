package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"video-smith/backend/internal/config"
	"video-smith/backend/internal/service/job"
	"video-smith/backend/internal/storage"
	"video-smith/backend/internal/utils"
	"video-smith/backend/internal/worker"
)

type Handlers struct {
	Config *config.Config
	Store  *storage.Store
	Queue  *worker.Queue
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (h *Handlers) CreateJob(w http.ResponseWriter, r *http.Request) {
	var req job.JobCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "請提供合法 JSON"})
		return
	}

	if err := req.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	record, err := job.NewJobRecord(req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := h.Store.InsertJob(record); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.Queue.Push(record.ID)
	writeJSON(w, http.StatusCreated, map[string]string{"id": record.ID})
}

func (h *Handlers) ListJobs(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	records, total, err := h.Store.ListJobs(page, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"page":  page,
		"limit": limit,
		"total": total,
		"data":  records,
	})
}

func (h *Handlers) GetJob(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	rec, err := h.Store.GetJob(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "找不到任務"})
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (h *Handlers) DownloadResult(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	rec, err := h.Store.GetJob(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "找不到任務"})
		return
	}
	if rec.Status != job.StatusSuccess {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "任務尚未完成"})
		return
	}
	fp := filepath.Join(rec.BasePath, "output.mp4")
	f, err := os.Open(fp)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "找不到輸出檔"})
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.mp4\"", id))
	_, _ = io.Copy(w, f)
}

func (h *Handlers) CancelJob(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	rec, err := h.Store.GetJob(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "找不到任務"})
		return
	}
	rec.Status = job.StatusCanceled
	rec.Progress = 0
	rec.ErrorMessage = "使用者取消"
	rec.UpdatedAt = time.Now()
	_ = h.Store.UpdateJob(rec)
	h.Queue.Cancel(id)
	_ = os.RemoveAll(rec.BasePath)
	writeJSON(w, http.StatusOK, map[string]string{"status": "canceled"})
}

func (h *Handlers) DeleteJob(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	rec, err := h.Store.GetJob(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "找不到任務"})
		return
	}
	_ = os.RemoveAll(rec.BasePath)
	_ = h.Store.DeleteJob(id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ListBGM 列出 preset 目錄可用音樂。
func (h *Handlers) ListBGM(w http.ResponseWriter, r *http.Request) {
	list := utils.ListAudioFiles(h.Config.BgmPath)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": list,
	})
}

// Swagger 傳回 OpenAPI JSON。
func (h *Handlers) Swagger(w http.ResponseWriter, r *http.Request) {
	specPath := filepath.Join("docs", "swagger.json")
	f, err := os.Open(specPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "swagger 檔案不存在"})
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.Copy(w, f)
}
