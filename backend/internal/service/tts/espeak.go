package tts

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"video-smith/backend/internal/utils"
)

// EspeakProvider 使用 espeak 合成，CPU-only，免費。
type EspeakProvider struct{}

func (e *EspeakProvider) Synthesize(text, voice, locale string, speed, pitch float64) (string, int, error) {
	if speed == 0 {
		speed = 1.0
	}
	tmpDir := os.TempDir()
	outPath := filepath.Join(tmpDir, "tts_"+strconv.FormatInt(int64(len(text)), 10)+".wav")
	wpm := int(175 * speed)
	args := []string{"-v", locale, "-s", fmt.Sprintf("%d", wpm), "-w", outPath, text}
	_, err := utils.RunCmd("espeak", args...)
	if err != nil {
		return "", 0, err
	}
	dur, _ := utils.AudioDurationMS(outPath)
	return outPath, dur, nil
}
