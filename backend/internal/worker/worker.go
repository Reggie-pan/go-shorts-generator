package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"video-smith/backend/internal/config"
	"video-smith/backend/internal/service/job"
	"video-smith/backend/internal/service/media"
	"video-smith/backend/internal/service/tts"
	"video-smith/backend/internal/storage"
	"video-smith/backend/internal/utils"
)

type Worker struct {
	cfg   *config.Config
	store *storage.Store
	queue *Queue
}

func NewWorker(cfg *config.Config, store *storage.Store, q *Queue) *Worker {
	return &Worker{cfg: cfg, store: store, queue: q}
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

	lines := utils.SplitScript(rec.Request.Script, rec.Request.SubtitleStyle.MaxLineWidth)
	for i, line := range lines {
		lines[i] = utils.AutoSpacing(line)
	}
	provider, err := tts.GetProvider(rec.Request.TTS.Provider, w.cfg)
	if err != nil {
		return err
	}
	var audioParts []string
	var durations []int
	log.Info().Str("job", rec.ID).Int("lines", len(lines)).Msg("開始 TTS 合成")
	for _, line := range lines {
		path, dur, err := provider.Synthesize(line, rec.Request.TTS.Voice, rec.Request.TTS.Locale, rec.Request.TTS.Speed, rec.Request.TTS.Pitch)
		if err != nil {
			return err
		}
		audioParts = append(audioParts, path)
		durations = append(durations, dur)
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
	if _, err := utils.RunCmd("ffmpeg", "-y", "-f", "concat", "-safe", "0", "-i", concatTxt, "-c", "copy", voiceOut); err != nil {
		return fmt.Errorf("合併語音失敗: %v", err)
	}
	totalVoiceDur, _ := utils.AudioDurationMS(voiceOut)
	log.Info().Str("job", rec.ID).Int("duration_ms", totalVoiceDur).Msg("語音合併完成")

	subs := utils.BuildTimeline(lines, durations)
	subLines := []media.SubtitleLine{}
	for _, s := range subs {
		subLines = append(subLines, media.SubtitleLine{
			Text:  s.Text,
			Start: s.Start,
			End:   s.End,
		})
	}
	subPath, err := media.BuildASS(base, rec.Request.SubtitleStyle, subLines)
	if err != nil {
		return err
	}

	rec.Progress = 55
	_ = w.store.UpdateJob(rec)

	videoSegments := media.BuildVideoTimeline(materials, rec.Request.Materials, totalVoiceDur)
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
	speed := rec.Request.Video.Speed
	if speed <= 0 {
		speed = 1.0
	}
	subPathFF := filepath.ToSlash(subPath)
	subPathFF = strings.ReplaceAll(subPathFF, "'", "\\'")
	videoFilter := fmt.Sprintf("subtitles='%s',setpts=%f*PTS", subPathFF, 1.0/speed)
	var args []string
	if bgmInput != "" {
		filter := fmt.Sprintf("[1:a]volume=%.2f,apad[b];[2:a]apad[v];[b][v]amix=inputs=2:duration=longest[aout]", rec.Request.BGM.Volume)
		args = []string{"-y", "-i", videoPath, "-i", bgmInput, "-i", voiceOut, "-filter_complex", filter, "-map", "0:v", "-map", "[aout]", "-c:v", "libx264", "-c:a", "aac", "-shortest", "-vf", videoFilter, output}
	} else {
		args = []string{"-y", "-i", videoPath, "-i", voiceOut, "-map", "0:v", "-map", "1:a", "-c:v", "libx264", "-c:a", "aac", "-shortest", "-vf", videoFilter, output}
	}
	if _, err := utils.RunCmd("ffmpeg", args...); err != nil {
		return fmt.Errorf("合成最終影片失敗: %v", err)
	}
	return nil
}
