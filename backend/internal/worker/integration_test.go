package worker

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"video-smith/backend/internal/config"
	"video-smith/backend/internal/service/job"
	"video-smith/backend/internal/storage"
)

// 需設 RUN_E2E=1 才執行，避免 CI 無 ffmpeg/espeak 時失敗。
func TestProcessPipeline(t *testing.T) {
	if os.Getenv("RUN_E2E") != "1" {
		t.Skip("E2E 測試預設略過，設 RUN_E2E=1 以啟用")
	}
	tmp, err := os.MkdirTemp("", "jobs")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	img1 := filepath.Join(tmp, "a.png")
	makeImage(img1, color.RGBA{255, 0, 0, 255})
	img2 := filepath.Join(tmp, "b.png")
	makeImage(img2, color.RGBA{0, 255, 0, 255})

	cfg := &config.Config{
		Port:        "0",
		StoragePath: tmp,
		BgmPath:     "",
	}
	store, err := storage.NewStore(tmp)
	if err != nil {
		t.Fatal(err)
	}
	q := NewQueue(1)
	w := NewWorker(cfg, store, q, nil)

	req := job.JobCreateRequest{
		Script: "這是一段測試腳本。第二句台詞。",
		Materials: []job.Material{
			{Type: "image", Source: "upload", PathOrURL: img1, DurationSec: 2},
			{Type: "image", Source: "upload", PathOrURL: img2, DurationSec: 2},
		},
		TTS:           job.TTSSetting{Provider: "free", Locale: "en", Speed: 1.0, Voice: ""},
		Video:         job.VideoSetting{Resolution: "720x1280", FPS: 25},
		BGM:           job.BGMSetting{Source: "", Volume: 0.0},
		SubtitleStyle: job.SubtitleStyle{Size: 28, Color: "FFFFFF", MaxLineWidth: 18},
	}
	rec, err := job.NewJobRecord(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.InsertJob(rec); err != nil {
		t.Fatal(err)
	}
	if err := w.process(rec); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(rec.BasePath, "output.mp4")); err != nil {
		t.Fatal("未產生輸出影片", err)
	}
}

func makeImage(path string, c color.Color) {
	img := image.NewRGBA(image.Rect(0, 0, 320, 240))
	for y := 0; y < 240; y++ {
		for x := 0; x < 320; x++ {
			img.Set(x, y, c)
		}
	}
	f, _ := os.Create(path)
	defer f.Close()
	_ = png.Encode(f, img)
}
