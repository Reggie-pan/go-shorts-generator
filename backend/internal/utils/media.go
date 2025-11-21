package utils

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type probeFormat struct {
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

// AudioDurationMS 使用 ffprobe 取得音訊長度。
func AudioDurationMS(path string) (int, error) {
	cmd := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json", "-show_format", path)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	var pf probeFormat
	if err := json.Unmarshal(out, &pf); err != nil {
		return 0, err
	}
	sec, err := strconv.ParseFloat(pf.Format.Duration, 64)
	if err != nil {
		return 0, err
	}
	return int(sec * 1000), nil
}

// PickFirstAudio 從指定目錄挑選第一個 mp3/wav 檔作為備援。
func PickFirstAudio(dir string) string {
	matches, _ := filepath.Glob(filepath.Join(dir, "*.mp3"))
	if len(matches) > 0 {
		return matches[0]
	}
	matches, _ = filepath.Glob(filepath.Join(dir, "*.wav"))
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

// ListAudioFiles 回傳目錄下 mp3/wav 檔名（basename）。
func ListAudioFiles(dir string) []string {
	files := []string{}
	entries, _ := filepath.Glob(filepath.Join(dir, "*"))
	for _, e := range entries {
		name := filepath.Base(e)
		low := strings.ToLower(name)
		if strings.HasSuffix(low, ".mp3") || strings.HasSuffix(low, ".wav") {
			files = append(files, name)
		}
	}
	return files
}
