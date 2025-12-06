package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
			log.Info().Str("job", rec.ID).Msg("任務處理完成")
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
	if _, err := utils.RunCmd("ffmpeg", "-y", "-f", "lavfi", "-i", "anullsrc=r=24000:cl=mono", "-t", "0.2", "-c:a", "pcm_s16le", silencePath); err != nil {
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

		// 2. 強制重編碼與修剪 (Re-encode & Trim)
		// 強制轉為 pcm_s16le 24000Hz mono，確保與靜音檔一致以便 concat 拼接
		trimmedPath := strings.TrimSuffix(path, filepath.Ext(path)) + fmt.Sprintf("_%d_processed.wav", i)
		filter := "silenceremove=start_periods=1:start_duration=0:start_threshold=-50dB:detection=peak,areverse,silenceremove=start_periods=1:start_duration=0:start_threshold=-50dB:detection=peak,areverse"

		if out, err := utils.RunCmd("ffmpeg", "-y", "-i", path, "-af", filter, "-c:a", "pcm_s16le", "-ar", "24000", "-ac", "1", trimmedPath); err != nil {
			log.Warn().Err(err).Str("output", out).Msg("音訊處理失敗，使用原始檔")
			trimmedPath = path
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
	// 合併語音，並再次重編碼，確保萬無一失
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

	// 4. 製作影片片段 (MakeSegments)
	// 這裡處理每個素材轉檔、縮放、加黑邊、合併音訊
	segments := media.BuildVideoTimeline(materials, rec.Request.Materials, int(totalVoiceDur*1000))
	log.Debug().Str("job", rec.ID).Str("resolution", rec.Request.Video.Resolution).Msg("製作影片片段")

	videoPath, err := media.MakeSegments(base, rec.Request.Video.Resolution, rec.Request.Video.FPS, rec.Request.Video.Background, segments, rec.Request.Video.Transition, rec.Request.Video.BlurBackground, func(percent int) {
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

	// 2. 準備背景音樂 (BGM)
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
			if _, err := utils.RunCmd("curl", "-L", "-o", bgmPath, rec.Request.BGM.Path); err != nil {
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

	output := filepath.Join(base, "output.mp4")

	subPathFF := filepath.ToSlash(subPath)
	subPathFF = strings.ReplaceAll(subPathFF, "'", "\\'")

	// 移除 setpts=PTS/%f，避免開始速度
	videoFilter := fmt.Sprintf("subtitles='%s'", subPathFF)

	var args []string
	log.Info().Str("job", rec.ID).Msg("執行 ffmpeg 合成")
	log.Info().Interface("subtitle_style", rec.Request.SubtitleStyle).Msg("Worker 使用字幕樣式")
	log.Info().Str("video_filter", videoFilter).Msg("套用 Video Filter")

	voiceSeconds := totalVoiceDur

	// 4. 最終合成
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

	// 最終音訊長度取 max(語音, 影片)
	// 這樣 BGM 才會播到影片結束
	finalDuration := voiceSeconds
	if videoDur > finalDuration {
		finalDuration = videoDur
	}

	if bgmInput != "" {
		// 3 inputs: VideoAudio, BGM, TTS
		// 改用 duration=longest，並且 BGM trim 到 finalDuration
		filter := fmt.Sprintf(`[0:v]%s[vout];[1:a]volume=%.2f,aloop=-1:size=0,atrim=0:%.3f,aformat=sample_rates=44100:channel_layouts=stereo[bgm];[2:a]atrim=0:%.3f,aformat=sample_rates=44100:channel_layouts=stereo[tts];[0:a]aformat=sample_rates=44100:channel_layouts=stereo[video_audio];[video_audio][bgm][tts]amix=inputs=3:duration=longest[aout]`,
			videoFilter, rec.Request.BGM.Volume, finalDuration, voiceSeconds)

		args = []string{"-y", "-i", videoPath, "-i", bgmInput, "-i", voiceOut, "-filter_complex", filter, "-map", "[vout]", "-map", "[aout]", "-c:v", "libx264", "-preset", "veryfast", "-c:a", "aac", "-shortest", output}
	} else {
		// 2 inputs: VideoAudio, TTS
		// 改用 duration=longest
		filter := fmt.Sprintf(`[0:v]%s[vout];[1:a]atrim=0:%.3f,aformat=sample_rates=44100:channel_layouts=stereo[tts];[0:a]aformat=sample_rates=44100:channel_layouts=stereo[video_audio];[video_audio][tts]amix=inputs=2:duration=longest[aout]`,
			videoFilter, voiceSeconds)

		args = []string{"-y", "-i", videoPath, "-i", voiceOut, "-filter_complex", filter, "-map", "[vout]", "-map", "[aout]", "-c:v", "libx264", "-preset", "veryfast", "-c:a", "aac", "-shortest", output}
	}
	if out, err := utils.RunCmdTimeout(5*time.Minute, "ffmpeg", args...); err != nil {
		return fmt.Errorf("合成最終影片失敗: %v / %s", err, out)
	} else if out != "" {
		log.Debug().Str("job", rec.ID).Msg(out)
	}
	rec.Progress = 95
	_ = w.store.UpdateJob(rec)
	return nil
}
