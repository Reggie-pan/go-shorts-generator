package worker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/Reggie-pan/go-shorts-generator/internal/ai"
	"github.com/Reggie-pan/go-shorts-generator/internal/config"
	"github.com/Reggie-pan/go-shorts-generator/internal/service/job"
	"github.com/Reggie-pan/go-shorts-generator/internal/service/media"
	"github.com/Reggie-pan/go-shorts-generator/internal/service/tts"
	"github.com/Reggie-pan/go-shorts-generator/internal/storage"
	"github.com/Reggie-pan/go-shorts-generator/internal/utils"
)

type Worker struct {
	cfg      *config.Config
	store    *storage.Store
	queue    *Queue
	aiClient *ai.Client
	wg       sync.WaitGroup
}

func NewWorker(cfg *config.Config, store *storage.Store, q *Queue, aiClient *ai.Client) *Worker {
	return &Worker{cfg: cfg, store: store, queue: q, aiClient: aiClient}
}

func (w *Worker) Run(ctx context.Context) {
	for {
		id, err := w.queue.PopWithContext(ctx)
		if err != nil {
			log.Info().Msg("Worker 停止接收新任務")
			return
		}
		w.wg.Add(1)
		w.processJobWithRecovery(ctx, id)
		w.wg.Done()
	}
}

// Wait 優雅關閉等待方法 (D1)
func (w *Worker) Wait(timeout time.Duration) {
	c := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(c)
	}()
	select {
	case <-c:
		log.Info().Msg("Worker 所有運行中任務優雅關閉完成")
	case <-time.After(timeout):
		log.Warn().Msg("Worker 優雅關閉超時，強制關閉")
	}
}

func (w *Worker) processJobWithRecovery(ctx context.Context, id string) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Str("job", id).Msg("Worker 遭遇嚴重 panic 崩潰")
			if rec, err := w.store.GetJob(id); err == nil {
				rec.Status = job.StatusFailed
				rec.ErrorMessage = fmt.Sprintf("系統發生未預期嚴重錯誤 (panic): %v", r)
				rec.Progress = 0
				rec.UpdatedAt = time.Now()
				_ = w.store.UpdateJob(rec)
			}
			w.queue.RemoveCancel(id)
		}
	}()

	if w.queue.IsCanceled(id) {
		w.queue.RemoveCancel(id)
		return
	}
	rec, err := w.store.GetJob(id)
	if err != nil {
		log.Error().Err(err).Str("job", id).Msg("讀取任務失敗")
		return
	}
	rec.Status = job.StatusRunning
	rec.Progress = 5
	rec.UpdatedAt = time.Now()
	_ = w.store.UpdateJob(rec)
	log.Info().Str("job", rec.ID).Msg("開始處理任務")
	if err := w.process(ctx, rec); err != nil {
		// 如果任務已被標記為取消，則不應該更新為失敗狀態
		if w.queue.IsCanceled(id) {
			log.Info().Str("job", id).Msg("任務已被手動取消，跳過失敗狀態更新")
			return
		}
		rec.Status = job.StatusFailed
		rec.ErrorMessage = err.Error()
		rec.Progress = 0
		log.Error().Str("job", rec.ID).Err(err).Msg("任務失敗")
	} else {
		rec.Status = job.StatusSuccess
		rec.Progress = 100
		rec.ResultURL = fmt.Sprintf("/api/v1/jobs/%s/result", rec.ID)
		log.Info().Str("job", rec.ID).Msg("任務處理完成")
	}
	rec.UpdatedAt = time.Now()
	_ = w.store.UpdateJob(rec)
	w.queue.RemoveCancel(id)
}

func (w *Worker) process(ctx context.Context, rec *job.Record) error {
	base := rec.BasePath
	if err := os.MkdirAll(base, 0o755); err != nil {
		return err
	}

	// 追蹤本輪產生的臨時語音片段檔案，以便在退出時清理 (D3)
	var tempFiles []string
	defer func() {
		for _, f := range tempFiles {
			if f != "" {
				_ = os.Remove(f)
			}
		}
	}()

	// D4: 提早將全域暫存區的上傳檔案移入該任務的 materials 目錄下，防止 temp 清理 API 提前刪除
	jobMaterialsDir := filepath.Join(base, "materials")
	_ = os.MkdirAll(jobMaterialsDir, 0o755)
	tmpDir := os.TempDir()
	for i, m := range rec.Request.Materials {
		if m.Source == "upload" && strings.HasPrefix(m.Path, tmpDir) {
			targetPath := filepath.Join(jobMaterialsDir, filepath.Base(m.Path))
			if _, err := os.Stat(m.Path); err == nil {
				if err := os.Rename(m.Path, targetPath); err == nil {
					rec.Request.Materials[i].Path = targetPath
					log.Info().Str("job", rec.ID).Str("from", m.Path).Str("to", targetPath).Msg("將暫存素材搬移到任務專屬目錄")
				} else {
					// 跨分割區時，Rename 會失敗，降級為複製並刪除
					if err := utils.CopyFile(m.Path, targetPath); err == nil {
						rec.Request.Materials[i].Path = targetPath
						_ = os.Remove(m.Path)
						log.Info().Str("job", rec.ID).Str("from", m.Path).Str("to", targetPath).Msg("跨分割區複製暫存素材")
					}
				}
			}
		}
	}

	log.Info().Str("job", rec.ID).Msg("準備素材")
	materials, err := media.PrepareMaterials(base, rec.Request.Materials)
	if err != nil {
		return err
	}
	rec.Progress = 15
	_ = w.store.UpdateJob(rec)

	log.Info().Str("job", rec.ID).Msg("AI 斷句中...")
	var lines []string
	if w.aiClient != nil {
		maxRetries := 3
		for i := 0; i <= maxRetries; i++ {
			lines, err = w.aiClient.SegmentText(rec.Request.Script, rec.Request.SubtitleStyle.MaxLineWidth)
			if err == nil {
				break
			}
			if i < maxRetries {
				log.Warn().Err(err).Int("retry", i+1).Msg("AI 斷句失敗，5秒後重試")
				time.Sleep(5 * time.Second)
			}
		}
	}
	if w.aiClient == nil || err != nil {
		if err != nil {
			log.Error().Err(err).Msg("AI 斷句失敗，降級使用規則斷句")
		} else {
			log.Info().Msg("無 AI 客戶端，使用規則斷句")
		}
		lines = utils.SplitScript(rec.Request.Script, rec.Request.SubtitleStyle.MaxLineWidth)
	}

	for i, line := range lines {
		lines[i] = utils.AutoSpacing(line)
	}
	provider, err := tts.GetProvider(rec.Request.TTS.Provider, w.cfg)
	if err != nil {
		return err
	}

	// 建立標準靜音檔 (0.2s, PCM 24k, Mono)
	silencePath := filepath.Join(base, "silence.wav")
	if _, err := utils.RunCmdContext(ctx, "ffmpeg", "-y", "-f", "lavfi", "-i", "anullsrc=r=24000:cl=mono", "-t", "0.2", "-c:a", "pcm_s16le", silencePath); err != nil {
		return fmt.Errorf("建立靜音檔失敗: %w", err)
	}
	silenceDur, _ := utils.AudioDurationSeconds(silencePath)

	var audioParts []string
	var durations []float64
	log.Info().Str("job", rec.ID).Int("lines", len(lines)).Msg("開始 TTS 合成")
	for i, line := range lines {
		// 1. 文本清洗 (Sanitization)
		ttsText := strings.ReplaceAll(line, "\n", ", ")
		ttsText = strings.ReplaceAll(ttsText, "\\", "")
		ttsText = strings.ReplaceAll(ttsText, "/", "")
		ttsText = strings.ReplaceAll(ttsText, "*", "")

		path, _, err := provider.Synthesize(ttsText, rec.Request.TTS.Voice, rec.Request.TTS.Locale, rec.Request.TTS.Speed, rec.Request.TTS.Pitch)
		if err != nil {
			return err
		}
		// 收集原始 TTS 暫存檔以供後續清理 (D3)
		tempFiles = append(tempFiles, path)

		// 2. 強制重編碼與修剪 (Re-encode & Trim)
		// 強制轉為 pcm_s16le 24000Hz mono，確保與靜音檔一致以便 concat 拼接
		trimmedPath := strings.TrimSuffix(path, filepath.Ext(path)) + fmt.Sprintf("_%d_processed.wav", i)
		// 修剪音訊頭部與尾部的靜音，確保說話句間停頓符合預設時長。
		// 由於 silenceremove 僅支援從開頭剪靜音，需配合 areverse 反轉音訊以修剪尾部。
		filter := "silenceremove=start_periods=1:start_duration=0:start_threshold=-50dB:detection=peak,areverse,silenceremove=start_periods=1:start_duration=0:start_threshold=-50dB:detection=peak,areverse"

		if out, err := utils.RunCmdContext(ctx, "ffmpeg", "-y", "-i", path, "-af", filter, "-c:a", "pcm_s16le", "-ar", "24000", "-ac", "1", trimmedPath); err != nil {
			log.Warn().Err(err).Str("output", out).Msg("音訊處理失敗，使用原始檔")
			trimmedPath = path
		} else {
			// 收集 trimmed 後的暫存檔 (D3)
			tempFiles = append(tempFiles, trimmedPath)
		}

		dur, _ := utils.AudioDurationSeconds(trimmedPath)

		// 3. 記錄片段與長度
		// 每句後面都加一段靜音
		audioParts = append(audioParts, trimmedPath)
		audioParts = append(audioParts, silencePath)

		// 字幕長度 = 語音長度 + 靜音長度
		// 這樣字幕會顯示直到下一句開始
		durations = append(durations, dur+silenceDur)

		// Update Progress: 15% -> 35%
		// TTS processing usually takes some time, so we update progress here.
		if len(lines) > 0 {
			currentProgress := 15 + int(float64(i+1)/float64(len(lines))*20)
			if currentProgress > 35 {
				currentProgress = 35
			}
			if currentProgress != rec.Progress {
				rec.Progress = currentProgress
				_ = w.store.UpdateJob(rec)
			}
		}
	}

	rec.Progress = 35
	_ = w.store.UpdateJob(rec)

	concatTxt := filepath.Join(base, "voice_list.txt")
	var list []string
	for _, p := range audioParts {
		list = append(list, fmt.Sprintf("file '%s'", p))
	}
	_ = os.WriteFile(concatTxt, []byte(strings.Join(list, "\n")), 0o644)

	voiceOut := filepath.Join(base, "voice.wav")
	// 合併語音，使用 copy 模式避免重編碼 (前面已統一格式)
	if out, err := utils.RunCmdContext(ctx, "ffmpeg", "-y", "-f", "concat", "-safe", "0", "-i", concatTxt, "-c:a", "copy", voiceOut); err != nil {
		return fmt.Errorf("合併語音失敗: %v / %s", err, out)
	}

	totalVoiceDur, _ := utils.AudioDurationSeconds(voiceOut)
	log.Info().Str("job", rec.ID).Float64("duration_sec", totalVoiceDur).Msg("語音合併完成")

	// 自動分配素材時長
	// 注意：BuildVideoTimeline 使用 needDuration + 1000 (語音長度 + 1 秒) 作為 hardLimit
	// 因此自動分配時也應該基於 totalVoiceDur + 1.0,確保素材能完整覆蓋影片時間軸
	if rec.Request.AutoDistributeDuration && len(rec.Request.Materials) > 0 {
		targetDuration := totalVoiceDur + 1.0 // 與 BuildVideoTimeline 的 hardLimit 一致
		avgDuration := targetDuration / float64(len(rec.Request.Materials))
		for i := range rec.Request.Materials {
			rec.Request.Materials[i].DurationSec = avgDuration
		}
		log.Info().Str("job", rec.ID).Float64("target_duration", targetDuration).Float64("avg_duration", avgDuration).Int("materials_count", len(rec.Request.Materials)).Msg("自動分配素材時長")
	}

	var sumDur float64
	for _, d := range durations {
		sumDur += d
	}
	log.Info().Str("job", rec.ID).Float64("sum_durations", sumDur).Float64("total_voice_dur", totalVoiceDur).Msg("時間軸校對")

	scaleFactor := 1.0
	if sumDur > 0 && totalVoiceDur > 0 {
		scaleFactor = totalVoiceDur / sumDur
	}
	log.Info().Str("job", rec.ID).Float64("scale_factor", scaleFactor).Msg("時間軸縮放")

	subs := utils.BuildTimelineFloat(lines, durations)
	subLines := []media.SubtitleLine{}
	for _, s := range subs {
		start := int(float64(s.Start) * scaleFactor)
		end := int(float64(s.End) * scaleFactor)
		subLines = append(subLines, media.SubtitleLine{
			Text:  s.Text,
			Start: start,
			End:   end,
		})
	}
	subPath, _, err := media.BuildASS(base, rec.Request.SubtitleStyle, subLines, rec.Request.Video.Resolution)
	if err != nil {
		return err
	}
	subPath, _ = filepath.Abs(subPath)

	// 4. 製作影片片段 (MakeSegments)
	// 這裡處理每個素材轉檔、縮放、加黑邊、合併音訊
	segments := media.BuildVideoTimeline(materials, rec.Request.Materials, int(totalVoiceDur*1000))
	log.Debug().Str("job", rec.ID).Str("resolution", rec.Request.Video.Resolution).Msg("製作影片片段")

	videoPath, err := media.MakeSegments(ctx, base, rec.Request.Video.Resolution, rec.Request.Video.FPS, rec.Request.Video.Background, segments, rec.Request.Video.Transition, rec.Request.Video.BlurBackground, w.cfg.FFmpegThreads, func(percent int) {
		// Video Generation: 35% -> 70%
		currentProgress := 35 + int(float64(percent)*0.35)
		if currentProgress > 70 {
			currentProgress = 70
		}
		if currentProgress != rec.Progress {
			rec.Progress = currentProgress
			_ = w.store.UpdateJob(rec)
		}
	})
	if err != nil {
		return fmt.Errorf("製作影片失敗: %v", err)
	}

	rec.Progress = 70
	_ = w.store.UpdateJob(rec)

	// 5. 封面處理 (Cover) - 只生成封面影片，稍後再拼接
	var coverVideoPath string
	if rec.Request.CoverStyle.Enabled {
		log.Info().Str("job", rec.ID).Msg("生成封面...")
		var coverVoicePath string
		var coverDuration float64

		if rec.Request.CoverStyle.TitleVoice {
			// 為標題生成語音
			titleVoicePath, _, err := provider.Synthesize(
				rec.Request.CoverStyle.Title,
				rec.Request.TTS.Voice,
				rec.Request.TTS.Locale,
				rec.Request.TTS.Speed,
				rec.Request.TTS.Pitch,
			)
			if err != nil {
				log.Warn().Err(err).Msg("生成封面標題語音失敗，使用預設時長")
				coverDuration = rec.Request.CoverStyle.Duration
			} else {
				coverVoicePath = titleVoicePath
				tempFiles = append(tempFiles, titleVoicePath) // 收集封面語音暫存檔以供清理 (D3)
				coverDuration, _ = utils.AudioDurationSeconds(titleVoicePath)
			}
		} else {
			coverDuration = rec.Request.CoverStyle.Duration
		}

		generatedCoverPath, err := media.GenerateCoverVideo(
			ctx,
			base,
			rec.Request.CoverStyle,
			rec.Request.SubtitleStyle,
			rec.Request.Video.Resolution,
			coverVoicePath,
			coverDuration,
			w.cfg.FFmpegThreads,
		)
		if err != nil {
			log.Warn().Err(err).Msg("生成封面失敗，跳過封面")
		} else {
			coverVideoPath = generatedCoverPath
			log.Info().Str("job", rec.ID).Msg("封面生成成功")
		}
		rec.Progress = 75
		_ = w.store.UpdateJob(rec)
	}

	// 準備背景音樂 (BGM)
	var bgmInput string
	if rec.Request.BGM.Source != "none" {
		bgmExt := filepath.Ext(rec.Request.BGM.Path)
		if bgmExt == "" {
			bgmExt = ".mp3"
		}
		bgmPath := filepath.Join(base, "bgm"+bgmExt)

		if rec.Request.BGM.Source == "preset" {
			// Preset: 從 assets/bgm 複製
			presetPath := filepath.Join(w.cfg.BgmPath, rec.Request.BGM.Path)
			if _, err := os.Stat(presetPath); os.IsNotExist(err) {
				return fmt.Errorf("BGM preset not found: %s", rec.Request.BGM.Path)
			}
			if err := utils.CopyFile(presetPath, bgmPath); err != nil {
				return fmt.Errorf("複製 BGM 失敗: %v", err)
			}
		} else if rec.Request.BGM.Source == "upload" {
			// Upload: 複製上傳檔案
			if err := utils.CopyFile(rec.Request.BGM.Path, bgmPath); err != nil {
				return fmt.Errorf("複製上傳 BGM 失敗: %v", err)
			}
		} else { // URL
			// URL: 下載
			if err := utils.DownloadFile(rec.Request.BGM.Path, bgmPath); err != nil {
				return fmt.Errorf("下載 BGM 失敗: %v", err)
			}
		}
		bgmInput = bgmPath

		// 檢查 BGM 檔案是否存在，若不存在則嘗試使用預設 BGM
		if _, err := os.Stat(bgmInput); err != nil {
			alt := utils.PickFirstAudio(w.cfg.BgmPath)
			if alt != "" {
				log.Warn().Str("job", rec.ID).Str("bgm", bgmInput).Str("fallback", alt).Msg("無法使用指定 BGM，改用預設")
				bgmInput = alt
			} else {
				log.Warn().Str("job", rec.ID).Str("bgm", bgmInput).Msg("無法使用 BGM，改為無背景音樂")
				bgmInput = ""
			}
		}
	}

	// 準備進度條圖片
	var progressBarInput string
	if rec.Request.ProgressBar.Enabled {
		imgExt := filepath.Ext(rec.Request.ProgressBar.ImagePath)
		if imgExt == "" {
			imgExt = ".png"
		}
		progressBarPath := filepath.Join(base, "progress_bar"+imgExt)
		if strings.HasPrefix(rec.Request.ProgressBar.ImagePath, "http://") || strings.HasPrefix(rec.Request.ProgressBar.ImagePath, "https://") {
			// URL: 下載
			if err := utils.DownloadFile(rec.Request.ProgressBar.ImagePath, progressBarPath); err != nil {
				log.Warn().Err(err).Msg("下載進度條圖片失敗")
			} else {
				progressBarInput = progressBarPath
			}
		} else {
			// 本地路徑: 複製
			if err := utils.CopyFile(rec.Request.ProgressBar.ImagePath, progressBarPath); err != nil {
				log.Warn().Err(err).Msg("複製進度條圖片失敗")
			} else {
				progressBarInput = progressBarPath
			}
		}
	}

	output := filepath.Join(base, "output.mp4")

	subPathFF := filepath.ToSlash(subPath)
	subPathFF = strings.ReplaceAll(subPathFF, "'", "\\'")

	// 構建字幕 filter
	videoFilter := fmt.Sprintf("subtitles='%s'", subPathFF)

	var args []string
	log.Info().Str("job", rec.ID).Msg("執行 ffmpeg 合成")
	log.Info().Interface("subtitle_style", rec.Request.SubtitleStyle).Msg("Worker 使用字幕樣式")
	log.Info().Str("video_filter", videoFilter).Msg("套用 Video Filter")

	voiceSeconds := totalVoiceDur

	// 最終合成影片與音訊
	// 輸入流
	// 0: video.mp4 (無聲 + 畫面)
	// 1: bgm.mp3 (可選)
	// 2: voice.wav (TTS)

	// 混音邏輯：
	// [0:a] 影片原音 (已在 MakeSegments 統一為 aac 44100 stereo)
	// [1:a] BGM
	// [2:a] TTS
	// 需要將 TTS (mono 24000) 轉為 stereo 44100 以便混音

	// 視頻邏輯：
	// [0:v] -> subtitles -> [vout]

	// 取得影片實際長度
	videoDur, err := utils.AudioDurationSeconds(videoPath)
	if err != nil {
		log.Warn().Err(err).Msg("無法取得影片實際長度，使用語音長度作為基準")
		videoDur = voiceSeconds
	}

	// 最終影片長度 = 語音長度 + 1 秒 (與 BuildVideoTimeline 的 hardLimit 一致)
	// 確保影片不會比語音長太多
	finalDuration := voiceSeconds + 1.0
	if videoDur < finalDuration {
		finalDuration = videoDur
	}

	if bgmInput != "" {
		// Inputs: 
		// 0: video, 1: bgm, 2: voice
		// 如果有進度條圖片，則是 0: video, 1: pbar, 2: bgm, 3: voice
		bgmIdx := "1:a"
		voiceIdx := "2:a"
		videoAudioIdx := "0:a"

		if progressBarInput != "" {
			bgmIdx = "2:a"
			voiceIdx = "3:a"
		}
		var pbarFilter string
		args, pbarFilter = media.BuildProgressBarFilter(progressBarInput, rec.Request.ProgressBar.Direction, finalDuration, videoPath, videoFilter)
		args = append(args, "-i", bgmInput, "-i", voiceOut)

		filter := fmt.Sprintf(`%s,trim=0:%.3f,setpts=PTS-STARTPTS[vout];[%s]volume=%.2f,aloop=-1:size=0,atrim=0:%.3f,aformat=sample_rates=44100:channel_layouts=stereo[bgm];[%s]aformat=sample_rates=44100:channel_layouts=stereo,volume=3.0,apad=whole_dur=%.3f[tts];[%s]atrim=0:%.3f,aformat=sample_rates=44100:channel_layouts=stereo[video_audio];[video_audio][bgm][tts]amix=inputs=3:duration=first[aout]`,
			pbarFilter, finalDuration, bgmIdx, rec.Request.BGM.Volume, finalDuration, voiceIdx, finalDuration, videoAudioIdx, finalDuration)

		args = append(args, "-filter_complex", filter, "-map", "[vout]", "-map", "[aout]", "-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-threads", w.cfg.FFmpegThreads, "-c:a", "aac", "-b:a", "128k", "-t", fmt.Sprintf("%.3f", finalDuration), output)
	} else {
		// Inputs:
		// 0: video, 1: voice
		// 如果有進度條圖片，則是 0: video, 1: pbar, 2: voice
		voiceIdx := "1:a"
		videoAudioIdx := "0:a"

		if progressBarInput != "" {
			voiceIdx = "2:a"
		}
		var pbarFilter string
		args, pbarFilter = media.BuildProgressBarFilter(progressBarInput, rec.Request.ProgressBar.Direction, finalDuration, videoPath, videoFilter)
		args = append(args, "-i", voiceOut)

		filter := fmt.Sprintf(`%s,trim=0:%.3f,setpts=PTS-STARTPTS[vout];[%s]aformat=sample_rates=44100:channel_layouts=stereo,volume=2.0,apad=whole_dur=%.3f[tts];[%s]atrim=0:%.3f,aformat=sample_rates=44100:channel_layouts=stereo[video_audio];[video_audio][tts]amix=inputs=2:duration=first[aout]`,
			pbarFilter, finalDuration, voiceIdx, finalDuration, videoAudioIdx, finalDuration)

		args = append(args, "-filter_complex", filter, "-map", "[vout]", "-map", "[aout]", "-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-threads", w.cfg.FFmpegThreads, "-c:a", "aac", "-b:a", "128k", "-t", fmt.Sprintf("%.3f", finalDuration), output)
	}
	if out, err := utils.RunCmdTimeoutContext(ctx, 5*time.Minute, "ffmpeg", args...); err != nil {
		return fmt.Errorf("合成最終影片失敗: %v / %s", err, out)
	} else if out != "" {
		log.Debug().Str("job", rec.ID).Msg(out)
	}
	rec.Progress = 90
	_ = w.store.UpdateJob(rec)

	// 如果有封面，在最終合成後拼接
	if coverVideoPath != "" {
		log.Info().Str("job", rec.ID).Msg("拼接封面到最終影片...")

		// 先重新編碼封面影片，確保與主影片格式完全一致
		reEncodedCover := filepath.Join(base, "cover_reencoded.mp4")
		if _, err := utils.RunCmdTimeoutContext(ctx, 2*time.Minute, "ffmpeg", "-y",
			"-i", coverVideoPath,
			"-vf", "setsar=1",
			"-c:v", "libx264", "-preset", "veryfast", "-crf", "23",
			"-threads", w.cfg.FFmpegThreads,
			"-c:a", "aac", "-b:a", "128k", "-ar", "44100", "-ac", "2",
			"-pix_fmt", "yuv420p",
			"-r", "30",
			reEncodedCover); err != nil {
			log.Warn().Err(err).Msg("重新編碼封面失敗")
			reEncodedCover = coverVideoPath
		}

		// 使用 concat demuxer 快速拼接（-c copy）
		concatListPath := filepath.Join(base, "concat_final.txt")
		concatContent := fmt.Sprintf("file '%s'\nfile '%s'\n", reEncodedCover, output)
		if err := os.WriteFile(concatListPath, []byte(concatContent), 0o644); err != nil {
			log.Warn().Err(err).Msg("寫入拼接列表失敗")
		} else {
			finalWithCover := filepath.Join(base, "final_with_cover.mp4")
			if _, err := utils.RunCmdTimeoutContext(ctx, 2*time.Minute, "ffmpeg", "-y",
				"-f", "concat", "-safe", "0", "-i", concatListPath,
				"-c", "copy",
				finalWithCover); err != nil {
				log.Warn().Err(err).Msg("拼接封面失敗，使用原始影片")
			} else {
				// 使用 ffmpeg 複製（比 cp/copy 更可靠）
				if _, err := utils.RunCmdTimeoutContext(ctx, 1*time.Minute, "ffmpeg", "-y",
					"-i", finalWithCover,
					"-c", "copy",
					output); err != nil {
					log.Warn().Err(err).Msg("複製最終影片失敗")
				} else {
					log.Info().Str("job", rec.ID).Msg("封面拼接成功")
				}
			}
		}
	}

	rec.Progress = 95
	_ = w.store.UpdateJob(rec)
	return nil
}
