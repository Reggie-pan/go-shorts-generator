package media

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Reggie-pan/go-shorts-generator/internal/utils"
	"github.com/rs/zerolog/log"
)

// ConvertAspectRatioRequest 轉換比例請求結構
type ConvertAspectRatioRequest struct {
	VideoPath       string `json:"video_path"`
	VideoURL        string `json:"video_url"`
	TargetW         int    `json:"width"`
	TargetH         int    `json:"height"`
	AspectRatio     string `json:"aspect_ratio"`      // 9:16, 16:9, 1:1, 4:5
	FillMode        string `json:"fill_mode"`         // color, blur, crop
	BackgroundColor string `json:"background_color"` // Hex 色碼 (如 #000000, #FF0000)，預設 #000000
}

// ParseResolution 依據傳入的 AspectRatio 或 Width/Height 計算出目標 Resolution
func ParseResolution(aspectRatio string, w, h int) (int, int) {
	if w > 0 && h > 0 {
		// 確保長寬為偶數 (FFmpeg libx264 要求)
		if w%2 != 0 {
			w++
		}
		if h%2 != 0 {
			h++
		}
		return w, h
	}

	ar := strings.TrimSpace(aspectRatio)
	resW, resH := 1080, 1920
	switch ar {
	case "16:9":
		resW, resH = 1920, 1080
	case "1:1":
		resW, resH = 1080, 1080
	case "4:5":
		resW, resH = 1080, 1350
	case "9:16":
		fallthrough
	default:
		resW, resH = 1080, 1920
	}

	if w > 0 && h <= 0 {
		resH = w * resH / resW
		resW = w
	} else if h > 0 && w <= 0 {
		resW = h * resW / resH
		resH = h
	}

	// 確保長寬為偶數 (FFmpeg libx264 要求)
	if resW%2 != 0 {
		resW++
	}
	if resH%2 != 0 {
		resH++
	}

	return resW, resH
}

// FormatFFmpegColor 將 Hex 色碼格式化為 FFmpeg 相容格式 (0xRRGGBB)
func FormatFFmpegColor(hex string) string {
	clean := strings.TrimSpace(hex)
	clean = strings.TrimPrefix(clean, "#")
	clean = strings.TrimPrefix(clean, "0x")
	clean = strings.TrimPrefix(clean, "0X")

	if len(clean) == 6 {
		return "0x" + strings.ToUpper(clean)
	}
	if len(clean) == 3 {
		// 短色碼如 F00 -> FF0000
		r := string(clean[0]) + string(clean[0])
		g := string(clean[1]) + string(clean[1])
		b := string(clean[2]) + string(clean[2])
		return "0x" + strings.ToUpper(r+g+b)
	}
	return "0x000000"
}

// BuildConvertAspectRatioFilter 構建 FFmpeg filter 參數與 filter_complex 語法
func BuildConvertAspectRatioFilter(fillMode, bgColor string, targetW, targetH int) (string, bool) {
	mode := strings.ToLower(strings.TrimSpace(fillMode))
	if mode == "" {
		mode = "color"
	}

	twStr := strconv.Itoa(targetW)
	thStr := strconv.Itoa(targetH)

	switch mode {
	case "crop":
		// 中央裁切：放大填滿目標長寬，裁切多餘邊界
		filter := fmt.Sprintf("scale=w='max(%[1]s,iw*%[2]s/ih)':h='max(%[2]s,ih*%[1]s/iw)':force_original_aspect_ratio=increase,crop=%[1]s:%[2]s", twStr, thStr)
		return filter, false

	case "blur":
		// 模糊影片當背景：背景降採樣為 360p 計算 boxblur 再放大，降低 CPU 運算負擔 (最適化 Synology DS920+ Celeron CPU)
		filter := fmt.Sprintf(
			"[0:v]scale=w='max(%[1]s,iw*%[2]s/ih)':h='max(%[2]s,ih*%[1]s/iw)':force_original_aspect_ratio=increase,crop=%[1]s:%[2]s,scale=360:-2,boxblur=20:5,scale=%[1]s:%[2]s[bg]; [0:v]scale=w='min(%[1]s,iw*%[2]s/ih)':h='min(%[2]s,ih*%[1]s/iw)':force_original_aspect_ratio=decrease[fg]; [bg][fg]overlay=(W-w)/2:(H-h)/2[v]",
			twStr, thStr,
		)
		return filter, true

	case "color":
		fallthrough
	default:
		// 補指定顏色：預設黑色 0x000000
		fmtColor := FormatFFmpegColor(bgColor)
		filter := fmt.Sprintf("scale=w='min(%[1]s,iw*%[2]s/ih)':h='min(%[2]s,ih*%[1]s/iw)':force_original_aspect_ratio=decrease,pad=%[1]s:%[2]s:(%[1]s-iw)/2:(%[2]s-ih)/2:color=%[3]s", twStr, thStr, fmtColor)
		return filter, false
	}
}

// ConvertVideoAspectRatio 呼叫 FFmpeg 執行影片比例轉換
func ConvertVideoAspectRatio(inputVideo, outputVideo, fillMode, bgColor string, targetW, targetH int, ffmpegThreads string) error {
	filterStr, isComplex := BuildConvertAspectRatioFilter(fillMode, bgColor, targetW, targetH)

	args := []string{
		"-y",
		"-i", inputVideo,
	}

	if isComplex {
		args = append(args,
			"-filter_complex", filterStr,
			"-map", "[v]",
			"-map", "0:a?",
		)
	} else {
		args = append(args,
			"-vf", filterStr,
			"-map", "0:v",
			"-map", "0:a?",
		)
	}

	args = append(args,
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "23",
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
	)

	if ffmpegThreads != "" {
		args = append(args, "-threads", ffmpegThreads)
	}

	args = append(args, outputVideo)

	log.Info().Str("input", inputVideo).Str("output", outputVideo).Str("filter", filterStr).Msg("開始執行 FFmpeg 影片比例轉換...")
	if _, err := utils.RunCmdTimeout(10*time.Minute, "ffmpeg", args...); err != nil {
		log.Error().Err(err).Msg("FFmpeg 影片比例轉換失敗")
		return fmt.Errorf("FFmpeg 影片比例轉換失敗: %w", err)
	}

	log.Info().Msg("FFmpeg 影片比例轉換完成")
	return nil
}

// ConvertVideoAspectRatioStream 執行影片比例轉換，將 Fragmented MP4 二進位串流即時推送到 outWriter (如 http.ResponseWriter)
func ConvertVideoAspectRatioStream(inputVideo string, outWriter io.Writer, fillMode, backgroundColor string, targetW, targetH int, ffmpegThreads string) error {
	filterStr, isComplex := BuildConvertAspectRatioFilter(fillMode, backgroundColor, targetW, targetH)

	args := []string{
		"-y",
		"-i", inputVideo,
	}

	if isComplex {
		args = append(args,
			"-filter_complex", filterStr,
			"-map", "[v]",
			"-map", "0:a?",
		)
	} else {
		args = append(args,
			"-vf", filterStr,
			"-map", "0:v",
			"-map", "0:a?",
		)
	}

	args = append(args,
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "23",
		"-c:a", "aac",
		"-b:a", "128k",
		"-f", "mp4",
		"-movflags", "empty_moov+frag_keyframe+default_base_moof",
	)

	if ffmpegThreads != "" {
		args = append(args, "-threads", ffmpegThreads)
	}

	// 輸出至 stdout (pipe:1)
	args = append(args, "pipe:1")

	log.Info().Str("input", inputVideo).Str("filter", filterStr).Msg("開始執行 FFmpeg 影片比例直推管道串流...")

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = outWriter
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Error().Err(err).Str("stderr", stderr.String()).Msg("FFmpeg 串流執行失敗")
		return fmt.Errorf("FFmpeg 串流執行失敗: %w, stderr: %s", err, stderr.String())
	}

	log.Info().Msg("FFmpeg 影片比例直推管道串流完成")
	return nil
}
