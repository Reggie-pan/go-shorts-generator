package api

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
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
			idx := rand.IntN(len(bgmList))
			selected := bgmList[idx]
			req.BGM.Path = selected
			log.Info().Str("selected_bgm", selected).Msg("隨機挑選 BGM")
		} else {
			log.Warn().Msg("隨機挑選 BGM 但無可用檔案，將不使用 BGM")
			req.BGM.Source = "none"
			req.BGM.Path = ""
		}
	}

	record, err := job.NewJobRecord(req, h.Config.StoragePath)
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
	// B6: 處理 UpdateJob 錯誤並記錄日誌
	if err := h.Store.UpdateJob(rec); err != nil {
		log.Error().Err(err).Str("job", id).Msg("取消任務時更新資料庫失敗")
	}
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

// Swagger 提供 OpenAPI JSON（支援多語言）
func (h *Handlers) Swagger(w http.ResponseWriter, r *http.Request) {
	// 根據 lang 參數決定檔名
	lang := r.URL.Query().Get("lang")
	filename := "swagger.json" // 預設繁體中文
	switch lang {
	case "zh-CN":
		filename = "swagger_cn.json"
	case "en":
		filename = "swagger_en.json"
	}

	// 嘗試多個可能的路徑
	possiblePaths := []string{
		filepath.Join("docs", filename),
		filepath.Join("/app", "docs", filename),
		filepath.Join("/app/docs", filename),
	}

	var f *os.File
	var err error
	for _, path := range possiblePaths {
		f, err = os.Open(path)
		if err == nil {
			break
		}
	}

	// 若找不到對應語言版本，回退至預設
	if f == nil && filename != "swagger.json" {
		fallbackPaths := []string{
			filepath.Join("docs", "swagger.json"),
			filepath.Join("/app", "docs", "swagger.json"),
			filepath.Join("/app/docs", "swagger.json"),
		}
		for _, path := range fallbackPaths {
			f, err = os.Open(path)
			if err == nil {
				break
			}
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
	// B4: 優化記憶體使用率，將 maxMemory 降至 32MB，大於 32MB 會自動暫存於硬碟
	if err := r.ParseMultipartForm(32 << 20); err != nil {
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

	// 使用專屬暫存目錄避免衝突
	tmpDir := filepath.Join(os.TempDir(), "go-shorts-generator")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		log.Error().Err(err).Msg("建立上傳暫存目錄失敗")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "建立暫存目錄失敗"})
		return
	}
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
	tmpDir := filepath.Join(os.TempDir(), "go-shorts-generator")
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, map[string]int{"deleted_count": 0})
			return
		}
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

// AddProgressBarOnly 接收現有影片與動態圖片設定，直接同步回傳合成後的影片
func (h *Handlers) AddProgressBarOnly(w http.ResponseWriter, r *http.Request) {
	var req job.AddProgressBarRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "請提供有效JSON"})
		return
	}

	log.Info().Interface("request", req).Msg("收到新增進度條請求")

	if req.VideoURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "請提供 video_url"})
		return
	}
	if req.ProgressBar.ImagePath == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "請提供 progress_bar.image_path"})
		return
	}

	// 1. 建立臨時目錄
	tmpDir, err := os.MkdirTemp("", "pbar_only_*")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "建立臨時目錄失敗: " + err.Error()})
		return
	}
	defer os.RemoveAll(tmpDir)

	localVideo := filepath.Join(tmpDir, "input.mp4")
	imgExt := filepath.Ext(req.ProgressBar.ImagePath)
	if imgExt == "" {
		imgExt = ".png"
	}
	// 避免 URL 參數帶有問號等髒資料，只取前綴的副檔名部分
	if idx := strings.Index(imgExt, "?"); idx != -1 {
		imgExt = imgExt[:idx]
	}
	localImg := filepath.Join(tmpDir, "pointer"+imgExt)
	outputVideo := filepath.Join(tmpDir, "output.mp4")

	// 2. 下載影片
	log.Info().Str("video_url", req.VideoURL).Msg("開始下載影片...")
	if strings.HasPrefix(req.VideoURL, "http://") || strings.HasPrefix(req.VideoURL, "https://") {
		if err := utils.DownloadFile(req.VideoURL, localVideo); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "下載來源影片失敗: " + err.Error()})
			return
		}
	} else {
		if err := utils.CopyFile(req.VideoURL, localVideo); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "讀取來源影片失敗: " + err.Error()})
			return
		}
	}

	// 3. 下載圖片
	log.Info().Str("image_url", req.ProgressBar.ImagePath).Msg("開始下載進度條圖片...")
	if strings.HasPrefix(req.ProgressBar.ImagePath, "http://") || strings.HasPrefix(req.ProgressBar.ImagePath, "https://") {
		if err := utils.DownloadFile(req.ProgressBar.ImagePath, localImg); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "下載進度條圖片失敗: " + err.Error()})
			return
		}
	} else {
		if err := utils.CopyFile(req.ProgressBar.ImagePath, localImg); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "讀取進度條圖片失敗: " + err.Error()})
			return
		}
	}

	// 4. 取得影片長度
	dur, err := utils.AudioDurationSeconds(localVideo)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "取得影片長度失敗: " + err.Error()})
		return
	}
	if dur <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "影片長度無效"})
		return
	}

	// 5. 利用 media 公用函式構建進度條的 FFmpeg 參數與 filter (D5)
	dir := req.ProgressBar.Direction
	if dir == "" {
		dir = "bottom"
	}
	args, filterComplex := media.BuildProgressBarFilter(localImg, dir, dur, localVideo, "")

	cmdArgs := append(args,
		"-filter_complex", filterComplex,
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "23",
		"-threads", h.Config.FFmpegThreads,
		"-c:a", "copy",
		outputVideo,
	)

	log.Info().Str("filter", filterComplex).Msg("開始執行 FFmpeg 合成進度條...")
	if _, err := utils.RunCmdTimeout(5*time.Minute, "ffmpeg", cmdArgs...); err != nil {
		log.Error().Err(err).Msg("FFmpeg 處理失敗")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "FFmpeg 處理失敗: " + err.Error()})
		return
	}

	log.Info().Msg("FFmpeg 進度條合成成功，開始回傳影片串流...")
	w.Header().Set("Content-Type", "video/mp4")
	http.ServeFile(w, r, outputVideo)
	log.Info().Msg("影片串流回傳完成！")
}

// ConvertAspectRatio 接收影片檔案或路徑，依指定比例與背景模式轉換影片比例
func (h *Handlers) ConvertAspectRatio(w http.ResponseWriter, r *http.Request) {
	var req media.ConvertAspectRatioRequest
	contentType := r.Header.Get("Content-Type")

	tmpDir, err := os.MkdirTemp("", "convert_ar_*")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "建立臨時目錄失敗: " + err.Error()})
		return
	}
	defer os.RemoveAll(tmpDir)

	localInput := filepath.Join(tmpDir, "input.mp4")
	localOutput := filepath.Join(tmpDir, "output.mp4")

	if strings.HasPrefix(contentType, "multipart/form-data") {
		_ = r.ParseMultipartForm(500 << 20) // 最大 500MB
		req.AspectRatio = r.FormValue("aspect_ratio")
		req.FillMode = r.FormValue("fill_mode")
		req.BackgroundColor = r.FormValue("background_color")
		if wStr := r.FormValue("width"); wStr != "" {
			req.TargetW, _ = strconv.Atoi(wStr)
		}
		if hStr := r.FormValue("height"); hStr != "" {
			req.TargetH, _ = strconv.Atoi(hStr)
		}
		req.VideoPath = r.FormValue("video_path")
		req.VideoURL = r.FormValue("video_url")

		file, header, err := r.FormFile("file")
		if err == nil && file != nil {
			defer file.Close()
			if req.VideoPath == "" {
				req.VideoPath = header.Filename
			}
			dst, err := os.Create(localInput)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "建立上傳檔案失敗: " + err.Error()})
				return
			}
			if _, err := io.Copy(dst, file); err != nil {
				dst.Close()
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "寫入上傳檔案失敗: " + err.Error()})
				return
			}
			dst.Close()
		}
	} else {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "請提供有效JSON或Form資料"})
			return
		}
	}

	log.Info().Interface("request", req).Msg("收到影片比例轉換請求")

	// 如果沒有經由 file 上傳，檢查是否有 video_url 或 video_path
	if _, err := os.Stat(localInput); os.IsNotExist(err) {
		source := req.VideoURL
		if source == "" {
			source = req.VideoPath
		}

		if source != "" {
			if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
				log.Info().Str("url", source).Msg("從 URL 下載來源影片...")
				if err := utils.DownloadFile(source, localInput); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "下載來源影片失敗: " + err.Error()})
					return
				}
			} else {
				log.Info().Str("path", source).Msg("從本機路徑讀取來源影片...")
				if err := utils.CopyFile(source, localInput); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "讀取來源影片失敗: " + err.Error()})
					return
				}
			}
		} else {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "請上傳影片檔案 (file) 或提供 video_url / video_path"})
			return
		}
	}

	targetW, targetH := media.ParseResolution(req.AspectRatio, req.TargetW, req.TargetH)
	log.Info().Int("targetW", targetW).Int("targetH", targetH).Str("fill_mode", req.FillMode).Msg("執行影片比例轉換...")

	// 做法 1 (同步串流早發標頭 TTFB 0ms)：
	// 在執行耗時 FFmpeg 轉碼前先發送 200 OK Response Header 給 Client (n8n)
	// 讓 Client Socket 立即進入「正在接收串流」狀態，避免轉碼期間 (30s+) 因 Socket 閒置而被切斷 (socket hang up)
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Content-Disposition", "inline; filename=\"converted_video.mp4\"")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	if err := media.ConvertVideoAspectRatio(localInput, localOutput, req.FillMode, req.BackgroundColor, targetW, targetH, h.Config.FFmpegThreads); err != nil {
		log.Error().Err(err).Msg("影片比例轉換失敗")
		return
	}

	outF, err := os.Open(localOutput)
	if err != nil {
		log.Error().Err(err).Msg("無法開啟轉換後影片檔案")
		return
	}
	defer outF.Close()

	if _, err := io.Copy(w, outF); err != nil {
		log.Error().Err(err).Msg("串流回傳影片檔案失敗")
		return
	}
	log.Info().Msg("影片串流回傳完成！")
}


