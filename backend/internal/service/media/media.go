package media

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
			out, err := utils.RunCmd("cp", m.PathOrURL, target)
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

		// 解析目標解析度
		w, h := 1080, 1920
		if parts := strings.Split(resolution, "x"); len(parts) == 2 {
			if v, err := strconv.Atoi(parts[0]); err == nil {
				w = v
			}
			if v, err := strconv.Atoi(parts[1]); err == nil {
				h = v
			}
		}

		// 建構 filter: 縮放並保持比例(decrease)，然後補黑邊置中，最後設定 SAR 為 1:1
		// scale=w:h:force_original_aspect_ratio=decrease
		// pad=w:h:(ow-iw)/2:(oh-ih)/2
		// setsar=1
		vf := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,setsar=1,fps=%d",
			w, h, w, h, fps)

		if seg.Type == "image" {
			if _, err := utils.RunCmd("ffmpeg", "-y", "-loop", "1", "-t", fmt.Sprintf("%.2f", durationSec), "-i", seg.Path,
				"-vf", vf, "-pix_fmt", "yuv420p", target); err != nil {
				return "", fmt.Errorf("製作圖片片段失敗: %v", err)
			}
		} else {
			if _, err := utils.RunCmd("ffmpeg", "-y", "-t", fmt.Sprintf("%.2f", durationSec), "-i", seg.Path,
				"-vf", vf, "-pix_fmt", "yuv420p", target); err != nil {
				return "", fmt.Errorf("裁切影片片段失敗: %v", err)
			}
		}
		list = append(list, fmt.Sprintf("file '%s'", target))
	}
	concatFile := filepath.Join(outDir, "list.txt")
	_ = os.WriteFile(concatFile, []byte(strings.Join(list, "\n")), 0o644)
	final := filepath.Join(base, "video.mp4")
	if _, err := utils.RunCmd("ffmpeg", "-y", "-f", "concat", "-safe", "0", "-i", concatFile, "-c", "copy", final); err != nil {
		return "", fmt.Errorf("合併片段失敗: %v", err)
	}
	return final, nil
}

// BuildASS 依樣式產生字幕檔，並根據解析度自動放大。
func BuildASS(base string, style job.SubtitleStyle, segments []SubtitleLine, resolution string) (string, job.SubtitleStyle, error) {
	_, resY := 1080, 1920
	if style.Font == "" {
		style.Font = "Noto Sans CJK TC"
	}
	if style.Color == "" {
		style.Color = "FFFFFF"
	}
	// 解析度處理：優先 request，次之環境變數 VIDEO_RESOLUTION
	if resolution == "" {
		resolution = os.Getenv("VIDEO_RESOLUTION")
	}
	if resolution != "" {
		if p := strings.Split(resolution, "x"); len(p) == 2 {
			if _, err := strconv.Atoi(p[0]); err == nil {
				// resX = w
			}
			if h, err := strconv.Atoi(p[1]); err == nil {
				resY = h
			}
		}
	}
	// 自動預設值（僅在未設定時）
	if style.Size <= 0 {
		style.Size = 48
	}
	if style.YOffset <= 0 {
		if resY >= 1200 {
			style.YOffset = 80
		} else if resY >= 720 {
			style.YOffset = 60
		} else {
			style.YOffset = 40
		}
	}
	color := strings.TrimPrefix(style.Color, "#")
	if len(color) == 6 {
		r := color[0:2]
		g := color[2:4]
		b := color[4:6]
		color = "00" + b + g + r // AA + BBGGRR
	}

	var b strings.Builder
	b.WriteString("[Script Info]\n")
	b.WriteString("ScriptType: v4.00+\n")
	// b.WriteString(fmt.Sprintf("PlayResX: %d\n", resX))
	// b.WriteString(fmt.Sprintf("PlayResY: %d\n", resY))
	b.WriteString("[V4+ Styles]\n")
	b.WriteString("Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	// BorderStyle=1，Outline=4，Shadow=1，Alignment=2（底中），MarginL/R=20，MarginV=YOffset
	b.WriteString(fmt.Sprintf("Style: Default,%s,%d,&H%s,&H00FFFFFF,&H00000000,&H64000000,0,0,0,0,100,100,0,0,1,4,1,2,20,20,%d,1\n", style.Font, style.Size, color, style.YOffset))
	b.WriteString("[Events]\n")
	b.WriteString("Format: Layer, Start, End, Style, Text\n")
	for _, seg := range segments {
		start := formatASSTime(seg.Start)
		end := formatASSTime(seg.End)
		text := strings.ReplaceAll(seg.Text, "\n", "\\N")
		b.WriteString(fmt.Sprintf("Dialogue: 0,%s,%s,Default,%s\n", start, end, text))
	}

	content := b.String()
	fmt.Printf("Generated ASS Content:\n%s\n", content)

	path := filepath.Join(base, "subtitle.ass")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", style, err
	}
	return path, style, nil
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
