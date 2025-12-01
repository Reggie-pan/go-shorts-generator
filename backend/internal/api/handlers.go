package api

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Reggie-pan/go-shorts-generator/internal/config"
	"github.com/Reggie-pan/go-shorts-generator/internal/service/job"
	"github.com/Reggie-pan/go-shorts-generator/internal/service/media"
	"github.com/Reggie-pan/go-shorts-generator/internal/service/tts"
	"github.com/Reggie-pan/go-shorts-generator/internal/storage"
	"github.com/Reggie-pan/go-shorts-generator/internal/utils"
	"github.com/Reggie-pan/go-shorts-generator/internal/worker"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
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
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "請提供有效JSON"})
		return
	}

	log.Info().Interface("request", req).Msg("收到建立任務請求")
	log.Info().Interface("subtitle_style", req.SubtitleStyle).Msg("收到字幕樣式參數")

	if err := req.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// 處理隨機 BGM
	if req.BGM.Source == "preset" && req.BGM.Path == "random" {
		bgmList := utils.ListAudioFiles(h.Config.BgmPath)
		if len(bgmList) > 0 {
			// 簡單隨機挑選
			// 注意：Go 1.20+ math/rand 自動 seed，若需要可能要手動 seed
			// 這裡簡單使用 time.Now().UnixNano() 做 seed
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			idx := rng.Intn(len(bgmList))
			selected := bgmList[idx]
			req.BGM.Path = selected
			log.Info().Str("selected_bgm", selected).Msg("隨機挑選 BGM")
		} else {
			log.Warn().Msg("隨機挑選 BGM 但無可用檔案，將不使用 BGM")
			req.BGM.Source = "none"
			req.BGM.Path = ""
		}
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
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "無法開啟輸出檔"})
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
	if err := os.RemoveAll(rec.BasePath); err != nil {
		log.Error().Err(err).Str("path", rec.BasePath).Msg("刪除任務目錄失敗")
	}
	_ = h.Store.DeleteJob(id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handlers) DeleteAllJobs(w http.ResponseWriter, r *http.Request) {
	// 1. 獲取所有任務以刪除檔案
	// 這裡假設數量不多，直接全取。若數量龐大可能需要分批處理
	// ListJobs page=1, limit=10000 (足夠大)
	records, _, err := h.Store.ListJobs(1, 10000)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "無法讀取任務列表"})
		return
	}

	// 2. 刪除檔案
	for _, rec := range records {
		if rec.BasePath != "" {
			if err := os.RemoveAll(rec.BasePath); err != nil {
				log.Error().Err(err).Str("path", rec.BasePath).Msg("刪除任務目錄失敗")
			}
		}
	}

	// 3. 清空資料庫
	if err := h.Store.DeleteAllJobs(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "清空資料庫失敗"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted_all"})
}

// ListBGM 列出 preset 可用的背景音樂
func (h *Handlers) ListBGM(w http.ResponseWriter, r *http.Request) {
	list := utils.ListAudioFiles(h.Config.BgmPath)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": list,
	})
}

// Swagger 提供 OpenAPI JSON
func (h *Handlers) Swagger(w http.ResponseWriter, r *http.Request) {
	// 嘗試多個可能的路徑
	possiblePaths := []string{
		filepath.Join("docs", "swagger.json"),
		filepath.Join("/app", "docs", "swagger.json"),
		filepath.Join("/app/docs", "swagger.json"),
	}

	var f *os.File
	var err error
	for _, path := range possiblePaths {
		f, err = os.Open(path)
		if err == nil {
			break
		}
	}

	if f == nil {
		log.Error().Err(err).Msg("找不到 swagger.json 檔案")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "swagger 檔案不存在"})
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_, _ = io.Copy(w, f)
}

type FontInfo struct {
	Name string `json:"name"`
}

var (
	fontCache      []FontInfo
	fontCacheMutex sync.RWMutex
)

// ListFonts 列出系統可用字型
func (h *Handlers) ListFonts(w http.ResponseWriter, r *http.Request) {
	// 檢查快取
	fontCacheMutex.RLock()
	if len(fontCache) > 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": fontCache,
		})
		fontCacheMutex.RUnlock()
		return
	}
	fontCacheMutex.RUnlock()

	// 1. 獲取字型列表
	// 使用 fc-list : family file 獲取字型名稱和路徑，以便確認是否為自定義字型
	cmd := exec.Command("fc-list", ":", "file", "family")
	output, err := cmd.Output()
	if err != nil {
		log.Error().Err(err).Msg("執行 fc-list 命令失敗")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "無法獲取字型列表"})
		return
	}

	// 定義常用西文字型列表 (用於過濾系統預設的大量字型)
	commonWesternFonts := map[string]bool{
		"Roboto": true, "Roboto Black": true, "Roboto Medium": true, "Roboto Light": true,
		"Ubuntu": true, "Ubuntu Mono": true, "Ubuntu Condensed": true,
		"Hack":           true,
		"Fira Code":      true,
		"JetBrains Mono": true,
		"Inconsolata":    true,
		"DejaVu Sans":    true, "DejaVu Serif": true, "DejaVu Sans Mono": true,
		"Liberation Sans": true, "Liberation Serif": true, "Liberation Mono": true,
		"Cantarell": true,
		"FreeSans":  true, "FreeSerif": true, "FreeMono": true,
		"Arial": true, "Times New Roman": true, "Courier New": true,
	}

	// 解析輸出結果
	fontLines := strings.Split(string(output), "\n")
	fontMap := make(map[string]bool)
	var fonts []FontInfo

	for _, line := range fontLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// fc-list output format: "file: family"
		// e.g. "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf: DejaVu Sans"
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		filePath := strings.TrimSpace(parts[0])
		familyStr := strings.TrimSpace(parts[1])

		// 可能有多個名稱，用逗號分隔
		families := strings.Split(familyStr, ",")

		// 尋找第一個英文名稱 (ASCII)
		var englishName string
		for _, family := range families {
			family = strings.TrimSpace(family)
			if family == "" {
				continue
			}

			isAscii := true
			for _, r := range family {
				if r > 127 {
					isAscii = false
					break
				}
			}

			if isAscii {
				englishName = family
				break
			}
		}

		// 如果沒有英文，或者值為空，就用第一個 (fallback)
		if englishName == "" && len(families) > 0 {
			englishName = strings.TrimSpace(families[0])
		}

		if englishName != "" && !fontMap[englishName] {
			keep := false

			// 1. 如果是自定義字型 (/assets/fonts -> /usr/share/fonts/custom)，絕對保留
			if strings.Contains(filePath, "/usr/share/fonts/custom") {
				keep = true
			} else {
				// 2. 系統字型過濾邏輯
				// 檢查是否為 CJK 字型 (簡單判斷名稱) 則保留
				upperName := strings.ToUpper(englishName)

				// 簡單判斷是否包含常見中文/CJK字型名稱關鍵字
				isCJK := strings.Contains(upperName, "CJK") ||
					strings.Contains(upperName, "HEI") ||
					strings.Contains(upperName, "MING") ||
					strings.Contains(upperName, "KAI") ||
					strings.Contains(upperName, "SANS") || // Noto Sans ...
					strings.Contains(upperName, "SERIF") // Noto Serif ...

				if isCJK {
					// 排除不需要的 CJK 變體，只保留常用的 TC/HK/TW
					if strings.Contains(upperName, "CN") ||
						strings.Contains(upperName, "SC") ||
						strings.Contains(upperName, "JP") ||
						strings.Contains(upperName, "KR") {
						keep = false
					} else {
						keep = true
					}
				} else {
					// 西文字型：只保留白名單中的
					for common := range commonWesternFonts {
						if strings.HasPrefix(englishName, common) {
							keep = true
							break
						}
					}
				}
			}

			if keep {
				fontMap[englishName] = true
				fonts = append(fonts, FontInfo{
					Name: englishName,
				})
			}
		}
	}

	// 字型排序
	sort.Slice(fonts, func(i, j int) bool {
		return fonts[i].Name < fonts[j].Name
	})

	// 更新快取
	fontCacheMutex.Lock()
	fontCache = fonts
	fontCacheMutex.Unlock()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": fonts,
	})
}

// PreviewSubtitleRequest 預覽請求結構
type PreviewSubtitleRequest struct {
	Text       string            `json:"text"`
	Style      job.SubtitleStyle `json:"style"`
	Background string            `json:"background"`
	Resolution string            `json:"resolution"`
}

// PreviewSubtitle 生成字幕預覽
func (h *Handlers) PreviewSubtitle(w http.ResponseWriter, r *http.Request) {
	var req PreviewSubtitleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "請提供有效JSON"})
		return
	}

	if req.Text == "" {
		req.Text = "預覽文字 Preview"
	}

	// 確保預設值
	if req.Style.Size == 0 {
		req.Style.Size = 16
	}
	if req.Style.Font == "" {
		req.Style.Font = "Noto Sans TC"
	}
	if req.Style.Color == "" {
		req.Style.Color = "FFFFFF"
	}
	if req.Style.OutlineWidth == 0 {
		req.Style.OutlineWidth = 0.1
	}
	if req.Style.OutlineColor == "" {
		req.Style.OutlineColor = "000000"
	}
	if req.Style.YOffset == 0 {
		req.Style.YOffset = 70
	}

	// 使用 media service 生成圖片
	// 輸出路徑使用系統預設暫存區
	tmpBase := os.TempDir()
	outPath, err := media.GeneratePreviewImage(tmpBase, req.Style, req.Text, req.Background, req.Resolution)
	if err != nil {
		log.Error().Err(err).Msg("生成預覽圖失敗")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 讀取並回傳圖片
	f, err := os.Open(outPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "無法讀取預覽圖"})
		return
	}
	defer f.Close()
	defer os.Remove(outPath) // 讀完後刪除

	w.Header().Set("Content-Type", "image/png")
	_, _ = io.Copy(w, f)
}

// UploadHandler 處理檔案上傳
func (h *Handlers) UploadHandler(w http.ResponseWriter, r *http.Request) {
	// 限制上傳大小為 500MB
	r.Body = http.MaxBytesReader(w, r.Body, 500<<20)
	if err := r.ParseMultipartForm(500 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "檔案太大或格式錯誤"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "無法讀取檔案"})
		return
	}
	defer file.Close()

	// 建立臨時檔案
	// 使用系統臨時目錄，並加上時間戳記避免衝突
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".bin"
	}

	// 確保 uploads 目錄存在 (可選，這裡直接用 TempDir)
	tmpDir := os.TempDir()
	dstName := fmt.Sprintf("upload_%d%s", time.Now().UnixNano(), ext)
	dstPath := filepath.Join(tmpDir, dstName)

	dst, err := os.Create(dstPath)
	if err != nil {
		log.Error().Err(err).Msg("建立上傳檔案失敗")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "建立檔案失敗"})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		log.Error().Err(err).Msg("寫入上傳檔案失敗")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "寫入檔案失敗"})
		return
	}

	// 回傳絕對路徑
	absPath, _ := filepath.Abs(dstPath)
	writeJSON(w, http.StatusOK, map[string]string{
		"path": absPath,
		"url":  "", // 暫時不提供 URL 訪問，僅供後端路徑使用
	})
}

// CleanTempFiles 清除所有暫存檔案
func (h *Handlers) CleanTempFiles(w http.ResponseWriter, r *http.Request) {
	tmpDir := os.TempDir()
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		log.Error().Err(err).Msg("讀取暫存目錄失敗")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "讀取暫存目錄失敗"})
		return
	}

	deletedCount := 0
	for _, f := range files {
		if !f.IsDir() && strings.HasPrefix(f.Name(), "upload_") {
			path := filepath.Join(tmpDir, f.Name())
			if err := os.Remove(path); err != nil {
				log.Warn().Err(err).Str("file", path).Msg("刪除暫存檔失敗")
			} else {
				deletedCount++
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]int{"deleted_count": deletedCount})
}

// ListVoices 列出 TTS 語音
func (h *Handlers) ListVoices(w http.ResponseWriter, r *http.Request) {
	providerName := r.URL.Query().Get("provider")
	if providerName == "" {
		providerName = "azure_v1"
	}

	p, err := tts.GetProvider(providerName, h.Config)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	voices, err := p.ListVoices()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": voices})
}
