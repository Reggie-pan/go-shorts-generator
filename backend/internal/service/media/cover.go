package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Reggie-pan/go-shorts-generator/internal/service/job"
	"github.com/Reggie-pan/go-shorts-generator/internal/utils"
	"github.com/rs/zerolog/log"
)

// GenerateCoverVideo 生成封面影片片段
// - ctx: Context
// - base: 工作目錄
// - coverStyle: 封面樣式設定
// - subtitleStyle: 字幕樣式（複用於標題文字）
// - resolution: 影片解析度
// - voicePath: 標題語音檔路徑（若無語音則傳空字串）
// - duration: 封面時長（秒）
// - threads: FFmpeg 執行緒數
// 返回封面影片路徑
func GenerateCoverVideo(ctx context.Context, base string, coverStyle job.CoverStyle, subtitleStyle job.SubtitleStyle, resolution string, voicePath string, duration float64, threads string) (string, error) {
	coverDir := filepath.Join(base, "cover")
	if err := os.MkdirAll(coverDir, 0o755); err != nil {
		return "", err
	}

	// 解析解析度
	w, h := 1080, 1920
	if parts := strings.Split(resolution, "x"); len(parts) == 2 {
		if v, err := strconv.Atoi(parts[0]); err == nil {
			w = v
		}
		if v, err := strconv.Atoi(parts[1]); err == nil {
			h = v
		}
	}

	// 1. 生成背景
	bgPath := filepath.Join(coverDir, "background.png")
	if err := generateBackground(ctx, bgPath, coverStyle, w, h); err != nil {
		return "", fmt.Errorf("生成封面背景失敗: %v", err)
	}

	// 2. 建立封面影片
	coverOutput := filepath.Join(coverDir, "cover.mp4")

	// 準備 drawtext 濾鏡（使用封面專屬的標題樣式）
	titleStyle := coverStyle.TitleStyle
	// 如果標題樣式未設定，使用傳入的字幕樣式作為預設
	if titleStyle.Size == 0 {
		titleStyle = subtitleStyle
	}
	titleFilter := buildTitleFilter(coverStyle.Title, titleStyle, w, h)

	// 設定 2 分鐘 timeout
	timeout := 2 * time.Minute

	if voicePath != "" {
		// 有語音：使用語音作為音訊，時長根據語音決定
		// 先處理語音：轉為 pcm_s16le 44100Hz stereo
		processedVoice := filepath.Join(coverDir, "voice_processed.wav")
		if _, err := utils.RunCmdTimeoutContext(ctx, timeout, "ffmpeg", "-y",
			"-i", voicePath,
			"-c:a", "pcm_s16le", "-ar", "44100", "-ac", "2",
			processedVoice); err != nil {
			return "", fmt.Errorf("處理封面語音失敗: %v", err)
		}

		// 取得語音時長
		voiceDur, err := utils.AudioDurationSeconds(processedVoice)
		if err != nil {
			log.Warn().Err(err).Msg("無法取得封面語音時長，使用預設時長")
			voiceDur = duration
		}

		// 加上延長時長
		extendDur := coverStyle.ExtendDuration
		if extendDur < 0 {
			extendDur = 0
		}
		totalDur := voiceDur + extendDur

		// 如果有延長時長，需要在語音後加入靜音
		if extendDur > 0 {
			// 生成延長靜音
			silencePath := filepath.Join(coverDir, "extend_silence.wav")
			if _, err := utils.RunCmdTimeoutContext(ctx, timeout, "ffmpeg", "-y",
				"-f", "lavfi", "-t", fmt.Sprintf("%.2f", extendDur), "-i", "anullsrc=r=44100:cl=stereo",
				"-c:a", "pcm_s16le",
				silencePath); err != nil {
				log.Warn().Err(err).Msg("生成延長靜音失敗")
			} else {
				// 合併語音和靜音
				mergedVoicePath := filepath.Join(coverDir, "voice_merged.wav")
				concatListPath := filepath.Join(coverDir, "voice_concat.txt")
				concatContent := fmt.Sprintf("file '%s'\nfile '%s'\n", processedVoice, silencePath)
				if err := os.WriteFile(concatListPath, []byte(concatContent), 0o644); err == nil {
					if _, err := utils.RunCmdTimeoutContext(ctx, timeout, "ffmpeg", "-y",
						"-f", "concat", "-safe", "0", "-i", concatListPath,
						"-c:a", "pcm_s16le", "-ar", "44100", "-ac", "2",
						mergedVoicePath); err == nil {
						processedVoice = mergedVoicePath
					}
				}
			}
		}

		// 生成封面影片
		filterComplex := fmt.Sprintf("[0:v]%s[vout]", titleFilter)
		if _, err := utils.RunCmdTimeoutContext(ctx, timeout, "ffmpeg", "-y",
			"-loop", "1", "-t", fmt.Sprintf("%.2f", totalDur), "-i", bgPath,
			"-i", processedVoice,
			"-filter_complex", filterComplex,
			"-map", "[vout]", "-map", "1:a",
			"-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-threads", threads,
			"-c:a", "aac", "-b:a", "128k", "-pix_fmt", "yuv420p", "-shortest",
			coverOutput); err != nil {
			return "", fmt.Errorf("生成封面影片失敗: %v", err)
		}
	} else {
		// 無語音：使用靜音，時長根據 duration 決定
		filterComplex := fmt.Sprintf("[0:v]%s[vout]", titleFilter)
		if _, err := utils.RunCmdTimeoutContext(ctx, timeout, "ffmpeg", "-y",
			"-loop", "1", "-t", fmt.Sprintf("%.2f", duration), "-i", bgPath,
			"-f", "lavfi", "-t", fmt.Sprintf("%.2f", duration), "-i", "anullsrc=r=44100:cl=stereo",
			"-filter_complex", filterComplex,
			"-map", "[vout]", "-map", "1:a",
			"-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-threads", threads,
			"-c:a", "aac", "-b:a", "128k", "-pix_fmt", "yuv420p", "-shortest",
			coverOutput); err != nil {
			return "", fmt.Errorf("生成封面影片失敗: %v", err)
		}
	}

	return coverOutput, nil
}

// generateBackground 根據設定生成背景圖片
func generateBackground(ctx context.Context, outputPath string, coverStyle job.CoverStyle, w, h int) error {
	timeout := 1 * time.Minute

	if coverStyle.BackgroundType == "image" {
		// 使用用戶圖片
		imgPath := coverStyle.BackgroundImage

		// 如果是 URL，先下載 (P2: 改用 Go 原生下載取代外部 curl 進程)
		if strings.HasPrefix(imgPath, "http://") || strings.HasPrefix(imgPath, "https://") {
			downloadPath := strings.TrimSuffix(outputPath, ".png") + "_download.jpg"
			if err := utils.DownloadFile(imgPath, downloadPath); err != nil {
				return fmt.Errorf("下載背景圖片失敗: %w", err)
			}
			imgPath = downloadPath
		}

		// 縮放到目標尺寸
		vf := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,setsar=1", w, h, w, h)

		// 如果需要模糊
		if coverStyle.BackgroundBlur {
			vf += ",boxblur=20:5"
		}

		if _, err := utils.RunCmdTimeoutContext(ctx, timeout, "ffmpeg", "-y",
			"-i", imgPath,
			"-vf", vf,
			"-frames:v", "1",
			outputPath); err != nil {
			return fmt.Errorf("處理背景圖片失敗: %v", err)
		}
	} else if coverStyle.BackgroundType == "solid" {
		// 單色背景
		colors := coverStyle.GradientColors
		bgColor := "FF6B9D"
		if len(colors) > 0 {
			bgColor = strings.TrimPrefix(colors[0], "#")
		}

		vf := fmt.Sprintf("color=c=0x%s:s=%dx%d:d=0.1,setsar=1", bgColor, w, h)
		if coverStyle.BackgroundBlur {
			vf += ",boxblur=20:5"
		}

		if _, err := utils.RunCmdTimeoutContext(ctx, timeout, "ffmpeg", "-y",
			"-f", "lavfi", "-i", vf,
			"-frames:v", "1",
			outputPath); err != nil {
			return fmt.Errorf("生成單色背景失敗: %v", err)
		}
	} else {
		// 漸層背景 (gradient)
		colors := coverStyle.GradientColors
		if len(colors) == 0 {
			colors = []string{"FF6B9D", "FFE66D", "4ECDC4"}
		}

		// 確保顏色格式正確（移除 #）
		for i, c := range colors {
			colors[i] = strings.TrimPrefix(c, "#")
		}

		// 使用 geq 濾鏡創建多色漸層
		// 對於垂直影片，我們創建一個從上到下的漸層
		c0 := colors[0]
		c1 := c0
		if len(colors) >= 2 {
			c1 = colors[len(colors)-1]
		}

		// 解析顏色為 RGB
		r0, g0, b0 := parseHexColor(c0)
		r1, g1, b1 := parseHexColor(c1)

		// 使用 geq 濾鏡創建漸層
		// r, g, b 根據 Y 座標線性插值
		geqFilter := fmt.Sprintf(
			"geq=r='%d+(Y/%d)*(%d-%d)':g='%d+(Y/%d)*(%d-%d)':b='%d+(Y/%d)*(%d-%d)'",
			r0, h, r1, r0,
			g0, h, g1, g0,
			b0, h, b1, b0,
		)

		vf := fmt.Sprintf("color=c=black:s=%dx%d:d=0.1,%s,setsar=1", w, h, geqFilter)
		if coverStyle.BackgroundBlur {
			vf += ",boxblur=10:3"
		}

		if _, err := utils.RunCmdTimeoutContext(ctx, timeout, "ffmpeg", "-y",
			"-f", "lavfi", "-i", vf,
			"-frames:v", "1",
			outputPath); err != nil {
			// 降級使用純色
			log.Warn().Err(err).Msg("漸層生成失敗，降級使用純色背景")
			bgColor := colors[0]
			simpleVf := fmt.Sprintf("color=c=0x%s:s=%dx%d:d=0.1,setsar=1", bgColor, w, h)
			if coverStyle.BackgroundBlur {
				simpleVf += ",boxblur=20:5"
			}
			if _, err := utils.RunCmdTimeoutContext(ctx, timeout, "ffmpeg", "-y",
				"-f", "lavfi", "-i", simpleVf,
				"-frames:v", "1",
				outputPath); err != nil {
				return fmt.Errorf("生成純色背景失敗: %v", err)
			}
		}
	}

	return nil
}

// parseHexColor 解析十六進位顏色為 RGB
func parseHexColor(hex string) (int, int, int) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 255, 107, 157 // 預設粉紅色
	}
	r, _ := strconv.ParseInt(hex[0:2], 16, 64)
	g, _ := strconv.ParseInt(hex[2:4], 16, 64)
	b, _ := strconv.ParseInt(hex[4:6], 16, 64)
	return int(r), int(g), int(b)
}

// buildTitleFilter 建立標題文字濾鏡 (支援多行每行皆置中對齊)
func buildTitleFilter(title string, style job.SubtitleStyle, w, h int) string {
	// 處理字型
	font := style.Font
	if font == "" {
		font = "Noto Sans CJK TC"
	}

	// 處理文字顏色
	color := strings.TrimPrefix(style.Color, "#")
	if color == "" {
		color = "FFFFFF"
	} else if len(color) < 6 {
		color = strings.Repeat("0", 6-len(color)) + color
	}

	// 處理描邊顏色
	outlineColor := strings.TrimPrefix(style.OutlineColor, "#")
	if outlineColor == "" {
		outlineColor = "000000"
	} else if len(outlineColor) < 6 {
		outlineColor = strings.Repeat("0", 6-len(outlineColor)) + outlineColor
	}

	// 處理字型大小
	fontSize := style.Size
	if fontSize <= 0 {
		fontSize = int(48 * 1.5)
	}

	// 處理描邊寬度
	outlineWidth := style.OutlineWidth
	if outlineWidth <= 0 {
		outlineWidth = 2
	}

	// 計算自動換行寬度
	maxLineWidth := float64(style.MaxLineWidth)
	fitCount := float64(w) * 0.90 / float64(fontSize)
	if fitCount < 2.0 {
		fitCount = 2.0
	}

	if maxLineWidth <= 0 || maxLineWidth > fitCount {
		maxLineWidth = fitCount
	}

	// 進行文字換行並切分行
	wrappedTitle := wrapText(title, maxLineWidth)
	lines := strings.Split(wrappedTitle, "\n")

	lineCount := len(lines)
	lineSpacing := int(float64(fontSize) * 0.15)
	if lineSpacing < 10 {
		lineSpacing = 10
	}
	totalH := lineCount*fontSize + (lineCount-1)*lineSpacing

	// 為每行產生獨立的 drawtext 濾鏡，確保每行文字都能獨立水平置中 x=(w-text_w)/2，並支援 y_offset 高度微調
	var filters []string
	for i, lineText := range lines {
		escapedLine := escapeDrawtext(lineText)
		yExpr := fmt.Sprintf("(h-%d)/2+%d", totalH, style.YOffset+i*(fontSize+lineSpacing))
		filter := fmt.Sprintf("drawtext=text='%s':expansion=none:font='%s':fontsize=%d:fontcolor=0x%s:borderw=%.1f:bordercolor=0x%s:x=(w-text_w)/2:y=%s",
			escapedLine, font, fontSize, color, outlineWidth, outlineColor, yExpr)
		filters = append(filters, filter)
	}

	return strings.Join(filters, ",")
}

// escapeDrawtext 跳脫 drawtext 濾鏡中的特殊字元
func escapeDrawtext(text string) string {
	// 跳脫單引號、反斜線、冒號、百分號
	text = strings.ReplaceAll(text, "\\", "\\\\")
	text = strings.ReplaceAll(text, "'", "'\\''")
	text = strings.ReplaceAll(text, ":", "\\:")
	text = strings.ReplaceAll(text, "%", "\\%")
	return text
}

