package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"video-smith/backend/internal/ai"
	"video-smith/backend/internal/config"
	"video-smith/backend/internal/service/job"
	"video-smith/backend/internal/service/media"
	"video-smith/backend/internal/service/tts"
	"video-smith/backend/internal/storage"
	"video-smith/backend/internal/utils"
)

type Worker struct {
	cfg      *config.Config
	store    *storage.Store
	queue    *Queue
	aiClient *ai.Client
}

func NewWorker(cfg *config.Config, store *storage.Store, q *Queue, aiClient *ai.Client) *Worker {
	return &Worker{cfg: cfg, store: store, queue: q, aiClient: aiClient}
}

func (w *Worker) Run() {
	for {
		id := w.queue.Pop()
		if w.queue.IsCanceled(id) {
			continue
		}
		rec, err := w.store.GetJob(id)
		if err != nil {
			log.Error().Err(err).Str("job", id).Msg("讀取任務失敗")
			continue
		}
		rec.Status = job.StatusRunning
		rec.Progress = 5
		rec.UpdatedAt = time.Now()
		_ = w.store.UpdateJob(rec)
		log.Info().Str("job", rec.ID).Msg("開始處理任務")
		if err := w.process(rec); err != nil {
			rec.Status = job.StatusFailed
			rec.ErrorMessage = err.Error()
			rec.Progress = 0
			log.Error().Str("job", rec.ID).Err(err).Msg("任務失敗")
		} else {
			rec.Status = job.StatusSuccess
			rec.Progress = 100
			rec.ResultURL = fmt.Sprintf("/api/v1/jobs/%s/result", rec.ID)
			log.Info().Str("job", rec.ID).Msg("任務成功完成")
		}
		rec.UpdatedAt = time.Now()
		_ = w.store.UpdateJob(rec)
	}
}

func (w *Worker) process(rec *job.Record) error {
	base := rec.BasePath
	if err := os.MkdirAll(base, 0o755); err != nil {
		return err
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
		lines, err = w.aiClient.SegmentText(rec.Request.Script, rec.Request.SubtitleStyle.MaxLineWidth)
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

	// 產生標準靜音檔 (0.2s, PCM 24k, Mono)
	silencePath := filepath.Join(base, "silence.wav")
	if _, err := utils.RunCmd("ffmpeg", "-y", "-f", "lavfi", "-i", "anullsrc=r=24000:cl=mono", "-t", "0.2", "-c:a", "pcm_s16le", silencePath); err != nil {
		return fmt.Errorf("建立靜音檔失敗: %w", err)
	}
	silenceDur, _ := utils.AudioDurationSeconds(silencePath)

	var audioParts []string
	var durations []float64
	log.Info().Str("job", rec.ID).Int("lines", len(lines)).Msg("開始 TTS 合成")
	for i, line := range lines {
		// 1. 文本清理 (Sanitization)
		ttsText := strings.ReplaceAll(line, "\n", ", ")
		ttsText = strings.ReplaceAll(ttsText, "\\", "")
		ttsText = strings.ReplaceAll(ttsText, "/", "")
		ttsText = strings.ReplaceAll(ttsText, "*", "")

		path, _, err := provider.Synthesize(ttsText, rec.Request.TTS.Voice, rec.Request.TTS.Locale, rec.Request.TTS.Speed, rec.Request.TTS.Pitch)
		if err != nil {
			return err
		}

		// 2. 強制重編碼與修剪 (Re-encode & Trim)
		// 強制轉為 pcm_s16le 24000Hz mono，確保與靜音檔一致，避免 concat 問題
		trimmedPath := strings.TrimSuffix(path, filepath.Ext(path)) + fmt.Sprintf("_%d_processed.wav", i)
		filter := "silenceremove=start_periods=1:start_duration=0:start_threshold=-50dB:detection=peak,areverse,silenceremove=start_periods=1:start_duration=0:start_threshold=-50dB:detection=peak,areverse"

		if _, err := utils.RunCmd("ffmpeg", "-y", "-i", path, "-af", filter, "-c:a", "pcm_s16le", "-ar", "24000", "-ac", "1", trimmedPath); err != nil {
			log.Warn().Err(err).Msg("音訊處理失敗，使用原始檔案")
			trimmedPath = path
		}

		dur, _ := utils.AudioDurationSeconds(trimmedPath)

		// 3. 記錄片段與長度
		// 每一句後面都接一個靜音檔
		audioParts = append(audioParts, trimmedPath)
		audioParts = append(audioParts, silencePath)

		// 字幕長度 = 語音長度 + 靜音長度
		// 這樣字幕會顯示直到下一句開始
		durations = append(durations, dur+silenceDur)
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
	// 合併時同樣強制重編碼，確保萬無一失
	if out, err := utils.RunCmd("ffmpeg", "-y", "-f", "concat", "-safe", "0", "-i", concatTxt, "-c:a", "pcm_s16le", "-ar", "24000", "-ac", "1", voiceOut); err != nil {
		return fmt.Errorf("合併語音失敗: %v / %s", err, out)
	}

	totalVoiceDur, _ := utils.AudioDurationSeconds(voiceOut)
	log.Info().Str("job", rec.ID).Float64("duration_sec", totalVoiceDur).Msg("語音合併完成")

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

	rec.Progress = 55
	_ = w.store.UpdateJob(rec)

	videoSegments := media.BuildVideoTimeline(materials, rec.Request.Materials, int(totalVoiceDur*1000))
	log.Debug().Str("job", rec.ID).Str("resolution", rec.Request.Video.Resolution).Msg("製作影片片段")
	videoPath, err := media.MakeSegments(base, rec.Request.Video.Resolution, rec.Request.Video.FPS, videoSegments)
	if err != nil {
		return err
	}

	rec.Progress = 70
	_ = w.store.UpdateJob(rec)

	bgmInput := ""
	if rec.Request.BGM.Source == "preset" && rec.Request.BGM.PathOrName == "" {
		bgmInput = filepath.Join(w.cfg.BgmPath, "default.mp3")
	} else if rec.Request.BGM.Source == "preset" {
		bgmInput = filepath.Join(w.cfg.BgmPath, filepath.Base(rec.Request.BGM.PathOrName))
	} else if rec.Request.BGM.Source == "url" {
		bgmInput = filepath.Join(base, "bgm.mp3")
		if _, err := utils.RunCmd("curl", "-L", "-o", bgmInput, rec.Request.BGM.PathOrName); err != nil {
			return fmt.Errorf("下載 BGM 失敗")
		}
	} else if rec.Request.BGM.Source == "upload" {
		bgmInput = rec.Request.BGM.PathOrName
	}
	if bgmInput != "" {
		if _, err := os.Stat(bgmInput); err != nil {
			alt := utils.PickFirstAudio(w.cfg.BgmPath)
			if alt != "" {
				log.Warn().Str("job", rec.ID).Str("bgm", bgmInput).Str("fallback", alt).Msg("找不到指定 BGM，改用備用")
				bgmInput = alt
			} else {
				log.Warn().Str("job", rec.ID).Str("bgm", bgmInput).Msg("找不到 BGM，改為無背景音樂")
				bgmInput = ""
			}
		}
	}

	output := filepath.Join(base, "output.mp4")

	subPathFF := filepath.ToSlash(subPath)
	subPathFF = strings.ReplaceAll(subPathFF, "'", "\\'")

	// 移除 setpts=PTS/%f，保持原始速度
	videoFilter := fmt.Sprintf("subtitles='%s'", subPathFF)

	var args []string
	log.Info().Str("job", rec.ID).Msg("開始 ffmpeg 合成")
	log.Info().Interface("subtitle_style", rec.Request.SubtitleStyle).Msg("Worker 使用的字幕樣式")
	log.Info().Str("video_filter", videoFilter).Msg("生成的 Video Filter")

	voiceSeconds := totalVoiceDur

	if bgmInput != "" {
		filter := fmt.Sprintf("[0:v]%s[vout];[1:a]volume=%.2f,aloop=-1:size=0,atrim=0:%.3f[b];[2:a]atrim=0:%.3f[v];[b][v]amix=inputs=2:duration=shortest[aout]",
			videoFilter, rec.Request.BGM.Volume, voiceSeconds, voiceSeconds)
		args = []string{"-y", "-i", videoPath, "-i", bgmInput, "-i", voiceOut, "-filter_complex", filter, "-map", "[vout]", "-map", "[aout]", "-c:v", "libx264", "-c:a", "aac", "-shortest", output}
	} else {
		filter := fmt.Sprintf("[0:v]%s[vout];[1:a]atrim=0:%.3f[aout]", videoFilter, voiceSeconds)
		args = []string{"-y", "-i", videoPath, "-i", voiceOut, "-filter_complex", filter, "-map", "[vout]", "-map", "[aout]", "-c:v", "libx264", "-c:a", "aac", "-shortest", output}
	}
	if out, err := utils.RunCmdTimeout(5*time.Minute, "ffmpeg", args...); err != nil {
		return fmt.Errorf("合成最終影片失敗 %v / %s", err, out)
	} else if out != "" {
		log.Debug().Str("job", rec.ID).Msg(out)
	}
	rec.Progress = 95
	_ = w.store.UpdateJob(rec)
	return nil
}
