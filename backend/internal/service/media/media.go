package media

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"video-smith/backend/internal/service/job"
	"video-smith/backend/internal/utils"
)

type SubtitleLine struct {
	Index int
	Start int
	End   int
	Text  string
}

// PrepareMaterials 下載或複製素材到工單資料夾。
func PrepareMaterials(base string, mats []job.Material) ([]string, error) {
	dir := filepath.Join(base, "materials")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var prepared []string
	for i, m := range mats {
		target := filepath.Join(dir, fmt.Sprintf("mat_%d", i))
		ext := filepath.Ext(m.PathOrURL)
		if ext == "" {
			if m.Type == "image" {
				ext = ".png"
			} else {
				ext = ".mp4"
			}
		}
		target = target + ext
		if strings.HasPrefix(m.PathOrURL, "http://") || strings.HasPrefix(m.PathOrURL, "https://") {
			out, err := utils.RunCmd("curl", "-L", "-o", target, m.PathOrURL)
			if err != nil {
				return nil, fmt.Errorf("下載素材失敗: %s %v", out, err)
			}
		} else {
			in := m.PathOrURL
			out, err := utils.RunCmd("cp", in, target)
			if err != nil {
				return nil, fmt.Errorf("複製素材失敗: %s %v", out, err)
			}
		}
		prepared = append(prepared, target)
	}
	return prepared, nil
}

// BuildVideoTimeline 依素材時長產生序列，若不足則補最後一個素材。
func BuildVideoTimeline(prepared []string, mats []job.Material, needDuration int) []Segment {
	var segments []Segment
	cursor := 0
	idx := 0
	for cursor < needDuration && idx < len(mats) {
		d := int(mats[idx].DurationSec * 1000)
		if d <= 0 {
			d = 1000
		}
		segments = append(segments, Segment{
			Path:  prepared[idx],
			Start: cursor,
			End:   cursor + d,
			Type:  mats[idx].Type,
		})
		cursor += d
		idx++
		if idx == len(mats) && cursor < needDuration {
			// 補最後一個素材至足夠
			last := mats[len(mats)-1]
			segments = append(segments, Segment{
				Path:  prepared[len(prepared)-1],
				Start: cursor,
				End:   cursor + (needDuration - cursor),
				Type:  last.Type,
			})
			cursor = needDuration
		}
	}
	return segments
}

type Segment struct {
	Path  string
	Start int
	End   int
	Type  string
}

// MakeSegments 產生影片片段並 concat。
func MakeSegments(base, resolution string, fps int, segments []Segment) (string, error) {
	outDir := filepath.Join(base, "segments")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	var list []string
	for i, seg := range segments {
		target := filepath.Join(outDir, fmt.Sprintf("seg_%d.mp4", i))
		durationSec := float64(seg.End-seg.Start) / 1000
		if seg.Type == "image" {
			_, err := utils.RunCmd("ffmpeg", "-y", "-loop", "1", "-t", fmt.Sprintf("%.2f", durationSec), "-i", seg.Path,
				"-vf", fmt.Sprintf("scale=%s,fps=%d", resolution, fps), "-pix_fmt", "yuv420p", target)
			if err != nil {
				return "", fmt.Errorf("製作圖片片段失敗: %v", err)
			}
		} else {
			_, err := utils.RunCmd("ffmpeg", "-y", "-t", fmt.Sprintf("%.2f", durationSec), "-i", seg.Path,
				"-vf", fmt.Sprintf("scale=%s,fps=%d", resolution, fps), "-pix_fmt", "yuv420p", target)
			if err != nil {
				return "", fmt.Errorf("裁切影片片段失敗: %v", err)
			}
		}
		list = append(list, fmt.Sprintf("file '%s'", target))
	}
	concatFile := filepath.Join(outDir, "list.txt")
	_ = os.WriteFile(concatFile, []byte(strings.Join(list, "\n")), 0o644)
	final := filepath.Join(base, "video.mp4")
	_, err := utils.RunCmd("ffmpeg", "-y", "-f", "concat", "-safe", "0", "-i", concatFile, "-c", "copy", final)
	if err != nil {
		return "", fmt.Errorf("合併片段失敗: %v", err)
	}
	return final, nil
}

// BuildASS 依樣式產生字幕檔。
func BuildASS(base string, style job.SubtitleStyle, segments []SubtitleLine) (string, error) {
	lines := []string{
		"[Script Info]",
		"ScriptType: v4.00+",
		"[V4+ Styles]",
		fmt.Sprintf("Style: Default,%s,%d,&H%s,0,0,0,0,100,100,0,0,1,2,2,2,10,%d,10,10,1", style.Font, style.Size, style.Color, style.YOffset),
		"[Events]",
		"Format: Layer, Start, End, Style, Text",
	}
	for _, seg := range segments {
		start := formatASSTime(seg.Start)
		end := formatASSTime(seg.End)
		text := strings.ReplaceAll(seg.Text, "\n", "\\N")
		lines = append(lines, fmt.Sprintf("Dialogue: 0,%s,%s,Default,%s", start, end, text))
	}
	path := filepath.Join(base, "subtitle.ass")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func formatASSTime(ms int) string {
	h := ms / 3600000
	ms = ms % 3600000
	m := ms / 60000
	ms = ms % 60000
	s := ms / 1000
	cs := (ms % 1000) / 10
	return fmt.Sprintf("%01d:%02d:%02d.%02d", h, m, s, cs)
}
