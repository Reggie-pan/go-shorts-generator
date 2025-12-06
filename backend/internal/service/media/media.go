package media

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Reggie-pan/go-shorts-generator/internal/service/job"
	"github.com/Reggie-pan/go-shorts-generator/internal/utils"
	"github.com/rs/zerolog/log"
)

type SubtitleLine struct {
	Start int
	End   int
	Text  string
}

// PrepareMaterials 下載素材並複製到工單資料夾
func PrepareMaterials(base string, mats []job.Material) ([]string, error) {
	dir := filepath.Join(base, "materials")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var prepared []string
	for i, m := range mats {
		target := filepath.Join(dir, fmt.Sprintf("mat_%d", i))
		ext := filepath.Ext(m.Path)
		if ext == "" {
			if m.Type == "image" {
				ext = ".png"
			} else {
				ext = ".mp4"
			}
		}
		target = target + ext
		m.Path = strings.TrimSpace(m.Path)
		if strings.HasPrefix(m.Path, "http://") || strings.HasPrefix(m.Path, "https://") {
			// 加入 -f (fail) 參數，下載 404/500 錯誤頁面
			out, err := utils.RunCmd("curl", "-f", "-L", "-o", target, m.Path)
			if err != nil {
				return nil, fmt.Errorf("下載素材失敗: %s %v", out, err)
			}
		} else if m.Source == "upload" {
			// 如果是上傳檔案，複製 (Copy) 到目標位置，保留原檔以備重試
			// 由 OS 或其他機制負責清理 /tmp
			if err := utils.CopyFile(m.Path, target); err != nil {
				return nil, fmt.Errorf("複製素材失敗: %v", err)
			}
		} else {
			if err := utils.CopyFile(m.Path, target); err != nil {
				return nil, fmt.Errorf("複製素材失敗: %v", err)
			}
		}
		prepared = append(prepared, target)
	}
	return prepared, nil
}

// BuildVideoTimeline 依據素材產生影片時間軸，補足或裁切以符合語音長度
// 修改：影片時間軸長度至少 1 秒 (hard limit)
func BuildVideoTimeline(prepared []string, mats []job.Material, needDuration int) []Segment {
	var segments []Segment
	cursor := 0
	idx := 0

	// 設定硬上限：語音長度 + 1秒
	hardLimit := needDuration + 1000

	for cursor < hardLimit {
		// 如果素材用完了，循環 (Loop)
		if idx >= len(mats) {
			idx = 0
		}

		d := int(mats[idx].DurationSec * 1000)
		if d <= 0 {
			d = 1000
		}

		// 檢查是否超過硬上限
		if cursor+d > hardLimit {
			d = hardLimit - cursor
		}

		// 如果裁斷後長度太短 (例如 < 0.5秒)，可以直接忽略或湊滿
		// 這裡簡單處理：只要 > 0 就加
		if d > 0 {
			segments = append(segments, Segment{
				Path:  prepared[idx],
				Start: cursor,
				End:   cursor + d,
				Type:  mats[idx].Type,
				Mute:  mats[idx].Mute,
			})
			cursor += d
		}

		// 如果已經達到硬上限就停止
		if cursor >= hardLimit {
			break
		}

		idx++
	}
	return segments
}

type Segment struct {
	Path  string
	Start int
	End   int
	Type  string
	Mute  bool
}

// MakeSegments 製作影片片段並 concat
func MakeSegments(base, resolution string, fps int, bgColor string, segments []Segment, transition string, blurBackground bool, onProgress func(int)) (string, error) {
	outDir := filepath.Join(base, "segments")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
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

	// 處理背景顏色，預設黑色
	if bgColor == "" {
		bgColor = "black"
	} else if !strings.HasPrefix(bgColor, "#") && len(bgColor) == 6 {
		bgColor = "#" + bgColor
	}

	// 建立 filter
	var vf string
	if blurBackground {
		// 模糊背景邏輯 (垂直影片)
		// 1. 背景層：縮放並填滿 (increase) -> Crop -> 縮小 (1/4) -> 模糊 -> 放大回原尺寸
		// 2. 前景層：縮放並適應 (decrease)
		// 3. Overlay
		vf = fmt.Sprintf("split[bg][fg];[bg]scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,scale=iw/4:-1,boxblur=10:5,scale=%d:%d:flags=neighbor[bg_blurred];[fg]scale=%d:%d:force_original_aspect_ratio=decrease[fg_scaled];[bg_blurred][fg_scaled]overlay=(W-w)/2:(H-h)/2,setsar=1,fps=%d",
			w, h, w, h, w, h, w, h, fps)
	} else {
		// 預設邏輯：黑邊
		vf = fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=%s,setsar=1,fps=%d",
			w, h, w, h, bgColor, fps)
	}

	var segmentFiles []string
	var durations []float64

	// 1. 處理每個片段
	for i, seg := range segments {
		target := filepath.Join(outDir, fmt.Sprintf("seg_%03d.mp4", i))

		// 計算 duration (ms -> sec)
		durationSec := float64(seg.End-seg.Start) / 1000.0

		// 設定重疊長度 (Overlap)
		// 為了避免轉場因檔案長度微小誤差而截斷，我們設定 1.2 秒的重疊區
		// 但轉場動畫 (xfade) 只使用 1.0 秒，保留 0.2 秒的安全緩衝
		const overlap = 1.2

		// 處理轉場，若不是最後一個片段，延長 overlap 秒給轉場使用
		if transition != "none" && transition != "" {
			if i < len(segments)-1 {
				durationSec += overlap
				// 中間片段需要足夠長以供重疊 (至少 overlap + 安全緩衝)
				if durationSec < overlap+1.0 {
					durationSec = overlap + 1.0
				}
			} else {
				// 最後一個片段雖然不用 fade out，但因為要 fade in，所以至少要 overlap 長度
				if durationSec < overlap {
					durationSec = overlap
				}
			}
		} else {
			// 預設模式，保險起見最小長度檢核 (可選)
			if durationSec < 0.1 {
				durationSec = 0.1
			}
		}

		// 設定 2 分鐘 timeout，避免卡死
		timeout := 2 * time.Minute

		// 準備 tpad filter 字串
		// 如果有轉場 (transition != "none")，我們需要確保影片最後一幀能延伸，
		// 以便與下一段影片進行重疊 (overlap)。
		// 這裡統一延伸 overlap 秒 (stop_duration=1.2)，模式為 clone (複製最後一幀)。
		var finalVf string
		if transition != "none" && transition != "" {
			// tpad=stop_mode=clone:stop_duration=1.2
			finalVf = fmt.Sprintf("%s,tpad=stop_mode=clone:stop_duration=%.1f", vf, overlap)
		} else {
			finalVf = vf
		}

		if seg.Type == "image" || seg.Mute {
			// 圖片或靜音影片：視訊流 + 靜音音訊
			if seg.Type == "image" {
				// 圖片：loop
				// 圖片本身就是靜態的，loop 足夠長即可，不需要 tpad，
				// 因為我們已經在 input 參數 -t 指定了 durationSec (含 overlap)。
				cmdArgs := []string{"-y",
					"-loop", "1", "-t", fmt.Sprintf("%.2f", durationSec), "-i", seg.Path,
					"-f", "lavfi", "-t", fmt.Sprintf("%.2f", durationSec), "-i", "anullsrc=r=44100:cl=stereo",
					"-filter_complex", fmt.Sprintf("[0:v]%s[v]", vf), // 圖片不需要 tpad，因為 loop 已經無限長
					"-map", "[v]", "-map", "1:a",
					"-c:v", "libx264", "-preset", "veryfast", "-c:a", "aac", "-pix_fmt", "yuv420p", "-shortest", target}

				if _, err := utils.RunCmdTimeout(timeout, "ffmpeg", cmdArgs...); err != nil {
					return "", fmt.Errorf("製作圖片片段失敗(seg %d): %v", i, err)
				}
			} else {
				// 影片(靜音)：只取畫面
				// 影片需要 tpad，因為影片長度是固定的。
				// 注意：durationSec 已經包含了 overlap (如果是中間片段)。
				// 我們希望影片播完後，停留在最後一幀直到 durationSec 結束。
				// 所以使用 finalVf (含 tpad)。
				if _, err := utils.RunCmdTimeout(timeout, "ffmpeg", "-y",
					"-t", fmt.Sprintf("%.2f", durationSec), "-i", seg.Path,
					"-f", "lavfi", "-t", fmt.Sprintf("%.2f", durationSec), "-i", "anullsrc=r=44100:cl=stereo",
					"-filter_complex", fmt.Sprintf("[0:v]%s[v]", finalVf),
					"-map", "[v]", "-map", "1:a",
					"-c:v", "libx264", "-preset", "veryfast", "-c:a", "aac", "-pix_fmt", "yuv420p", "-shortest", target); err != nil {
					return "", fmt.Errorf("製作影片片段(靜音 seg %d)失敗: %v", i, err)
				}
			}
		} else {
			// 影片(有聲)：畫面 + 重編碼音訊
			// 同樣需要 tpad
			// 音訊部分：如果影片長度不夠，音訊會變短，導致 output 變短。
			// 我們需要 pad 音訊嗎？
			// 轉場時通常希望聲音 crossfade。如果影片沒聲音了，補靜音即可。
			// aformat 之後加入 apad？
			// 這裡先處理視頻 tpad。音訊部分若短於視頻，ffmpeg -shortest 會以最短的為準嗎？
			// 我們指定了 -t durationSec，所以 ffmpeg 會嘗試輸出這麼長。
			// 如果音訊不夠長，apad 可以補靜音。

			// 構建 filter complex
			// [0:v] -> finalVf -> [v]
			// [0:a] -> aformat -> apad -> [a]

			filterComplex := fmt.Sprintf("[0:v]%s[v];[0:a]aformat=sample_rates=44100:channel_layouts=stereo,apad[a]", finalVf)

			if _, err := utils.RunCmdTimeout(timeout, "ffmpeg", "-y",
				"-t", fmt.Sprintf("%.2f", durationSec), "-i", seg.Path,
				"-filter_complex", filterComplex,
				"-map", "[v]", "-map", "[a]",
				"-c:v", "libx264", "-preset", "veryfast", "-c:a", "aac", "-pix_fmt", "yuv420p", "-shortest", target); err != nil {

				// 若失敗，可能是沒有音軌，嘗試靜音模式重試
				if _, err := utils.RunCmdTimeout(timeout, "ffmpeg", "-y",
					"-t", fmt.Sprintf("%.2f", durationSec), "-i", seg.Path,
					"-f", "lavfi", "-t", fmt.Sprintf("%.2f", durationSec), "-i", "anullsrc=r=44100:cl=stereo",
					"-filter_complex", fmt.Sprintf("[0:v]%s[v]", finalVf),
					"-map", "[v]", "-map", "1:a",
					"-c:v", "libx264", "-preset", "veryfast", "-c:a", "aac", "-pix_fmt", "yuv420p", "-shortest", target); err != nil {
					return "", fmt.Errorf("製作影片片段失敗(重試靜音 seg %d): %v", i, err)
				}
			}
		}

		// 取得實際生成的檔案長度，以確保轉場 offset 計算精確
		actualDur, err := utils.AudioDurationSeconds(target)
		if err != nil {
			// 若無法取得，fallback 回原本計算的 durationSec
			log.Warn().Err(err).Str("file", target).Msg("無法取得片段實際長度，使用預估值")
		} else {
			// update durationSec to actual duration
			// 注意：這裡的 actualDur 應該非常接近 durationSec (或是因為 tpad 而變長)
			// 我們信任實際檔案長度
			durationSec = actualDur
		}

		segmentFiles = append(segmentFiles, target)
		durations = append(durations, durationSec)

		if onProgress != nil {
			percent := int(float64(i+1) / float64(len(segments)+1) * 100)
			onProgress(percent)
		}
	}

	final := filepath.Join(base, "video.mp4")

	// 2. 合併片段
	if transition == "none" || transition == "" || len(segmentFiles) < 2 {
		// 預設模式使用 concat demuxer (快速)
		var list []string
		for _, f := range segmentFiles {
			list = append(list, fmt.Sprintf("file '%s'", f))
		}
		concatFile := filepath.Join(outDir, "list.txt")
		_ = os.WriteFile(concatFile, []byte(strings.Join(list, "\n")), 0o644)

		if _, err := utils.RunCmdTimeout(2*time.Minute, "ffmpeg", "-y", "-f", "concat", "-safe", "0", "-i", concatFile, "-c", "copy", final); err != nil {
			return "", fmt.Errorf("合併片段失敗: %v", err)
		}
	} else {
		// 轉場模式使用 xfade (重新編碼)
		// 轉場長度固定 1 秒
		offset := 0.0
		filterComplex := ""
		const overlap = 1.2

		// 構建 filter_complex
		// [0][1]xfade=transition=fade:duration=1:offset=OFFSET[v1];
		// [0:a][1:a]acrossfade=d=1.2[a1];
		// [v1][2]xfade...

		// 視訊流
		prevV := "0:v"
		prevA := "0:a"

		for i := 0; i < len(segmentFiles)-1; i++ {
			nextV := fmt.Sprintf("%d:v", i+1)
			nextA := fmt.Sprintf("%d:a", i+1)

			// 計算 offset: 上一段 offset + 上一段長度 - 重疊長度
			// 第一個 offset = duration[0] - overlap
			// 第二個 offset = duration[0] + duration[1] - 2*overlap ?
			// 修正：offset 是相對於前一個影片的開始
			// offset += currentDur - overlap

			currentDur := durations[i]
			if i == 0 {
				offset = currentDur - overlap
			} else {
				offset += currentDur - overlap
			}

			outV := fmt.Sprintf("v%d", i+1)
			outA := fmt.Sprintf("a%d", i+1)

			if i == len(segmentFiles)-2 {
				outV = "outv"
				outA = "outa"
			}

			// xfade duration 保持 1.0，但 offset 提早到 overlap (1.2)
			// 這樣有 0.2s 的安全緩衝
			filterComplex += fmt.Sprintf("[%s][%s]xfade=transition=%s:duration=1:offset=%.2f[%s];", prevV, nextV, transition, offset, outV)
			// acrossfade duration 必須等於 overlap (1.2)，以保持音訊同步
			filterComplex += fmt.Sprintf("[%s][%s]acrossfade=d=%.2f[%s];", prevA, nextA, overlap, outA)

			prevV = outV
			prevA = outA
		}
		// 移除最後的分號
		filterComplex = strings.TrimSuffix(filterComplex, ";")

		// 執行 ffmpeg
		// 注意：inputs 包含多個 -i，需要正確拼接
		args := []string{"-y"}
		for _, f := range segmentFiles {
			args = append(args, "-i", f)
		}
		args = append(args, "-filter_complex", filterComplex, "-map", "[outv]", "-map", "[outa]", "-c:v", "libx264", "-preset", "veryfast", "-c:a", "aac", "-pix_fmt", "yuv420p", final)

		// Debug Log
		fmt.Printf("Transition FFmpeg Args: %v\n", args)

		if out, err := utils.RunCmdTimeout(10*time.Minute, "ffmpeg", args...); err != nil {
			return "", fmt.Errorf("轉場合併失敗: %v, Output: %s", err, out)
		}
	}

	return final, nil
}

// BuildASS 依樣式產生字幕檔，並處理高度自動放大
func BuildASS(base string, style job.SubtitleStyle, segments []SubtitleLine, resolution string) (string, job.SubtitleStyle, error) {
	_, resY := 1080, 1920
	if style.Font == "" {
		style.Font = "Noto Sans CJK TC"
	}
	if style.Color == "" {
		style.Color = "FFFFFF"
	}
	// 解析度優先使用 request，次之環境變數 VIDEO_RESOLUTION
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
	// 處理預設值與在線設定修正
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

	outlineColor := strings.TrimPrefix(style.OutlineColor, "#")
	if len(outlineColor) == 6 {
		r := outlineColor[0:2]
		g := outlineColor[2:4]
		b := outlineColor[4:6]
		outlineColor = "00" + b + g + r // AA + BBGGRR
	} else {
		outlineColor = "00000000"
	}

	var b strings.Builder
	b.WriteString("[Script Info]\n")
	b.WriteString("ScriptType: v4.00+\n")
	// b.WriteString(fmt.Sprintf("PlayResX: %d\n", resX))
	// b.WriteString(fmt.Sprintf("PlayResY: %d\n", resY))
	b.WriteString("[V4+ Styles]\n")
	b.WriteString("Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	// BorderStyle=1，Outline=style.OutlineWidth，Shadow=0，Alignment=2（正中央），MarginL/R=20，MarginV=YOffset
	b.WriteString(fmt.Sprintf("Style: Default,%s,%d,&H%s,&H00FFFFFF,&H%s,&H64000000,0,0,0,0,100,100,0,0,1,%.1f,0,2,20,20,%d,1\n", style.Font, style.Size, color, outlineColor, style.OutlineWidth, style.YOffset))
	b.WriteString("[Events]\n")
	b.WriteString("Format: Layer, Start, End, Style, Text\n")
	for _, seg := range segments {
		start := formatASSTime(seg.Start)
		end := formatASSTime(seg.End)

		// 自動換行邏輯：如果文本超過 max_line_width，插入 \N 換行符
		text := seg.Text
		if style.MaxLineWidth > 0 {
			text = wrapText(text, style.MaxLineWidth)
		}

		// 替換換行符為 ASS 格式
		text = strings.ReplaceAll(text, "\n", "\\N")
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

// wrapText 將文本按照最大寬度自動換行
func wrapText(text string, maxWidth int) string {
	runes := []rune(text)
	if utf8.RuneCountInString(text) <= maxWidth {
		return text
	}

	var lines []string
	var currentLine []rune

	for i := 0; i < len(runes); {
		// 嘗試取出 maxWidth 個字符
		remaining := len(runes) - i
		chunkSize := maxWidth
		if remaining < chunkSize {
			chunkSize = remaining
		}

		chunk := runes[i : i+chunkSize]

		// 如果這是最後一塊，直接加入
		if i+chunkSize >= len(runes) {
			currentLine = append(currentLine, chunk...)
			lines = append(lines, string(currentLine))
			break
		}

		// 檢查是否可以在更好的位置切分（空格、標點等）
		// 向後查找最多 8 個字符
		bestSplit := chunkSize
		foundPunctuation := false
		for j := chunkSize; j > chunkSize-8 && j > 0; j-- {
			char := runes[i+j-1]
			// 中文標點或空格
			if char == ' ' || char == '，' || char == '。' || char == '！' || char == '？' || char == '；' || char == '、' {
				bestSplit = j
				foundPunctuation = true
				break
			}
		}

		// 如果沒找到標點/空格，嘗試找一個"安全"的切分點 (非英數/特殊符號)
		if !foundPunctuation {
			for j := chunkSize; j > chunkSize-8 && j > 0; j-- {
				idx := i + j
				if idx >= len(runes) {
					continue
				}

				curr := runes[idx]
				prev := runes[idx-1]

				isUnsafe := false
				// 1. 英文/數字中間
				isPrevAlpha := (prev >= 'a' && prev <= 'z') || (prev >= 'A' && prev <= 'Z') || (prev >= '0' && prev <= '9')
				isCurrAlpha := (curr >= 'a' && curr <= 'z') || (curr >= 'A' && curr <= 'Z') || (curr >= '0' && curr <= '9')
				if isPrevAlpha && isCurrAlpha {
					isUnsafe = true
				}

				// 2. 特殊符號前
				if curr == '%' || curr == '％' || curr == '℃' || curr == '°' {
					isUnsafe = true
				}

				if !isUnsafe {
					bestSplit = j
					break
				}
			}
		}

		currentLine = append(currentLine, runes[i:i+bestSplit]...)
		lines = append(lines, string(currentLine))
		currentLine = []rune{}
		i += bestSplit
	}

	return strings.Join(lines, "\n")
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

// GeneratePreviewImage 產生字幕預覽圖
func GeneratePreviewImage(base string, style job.SubtitleStyle, text string, bgColor string, resolution string) (string, error) {
	// 建立臨時目錄
	tmpDir := filepath.Join(base, "preview")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", err
	}

	// 預設解析度
	if resolution == "" {
		resolution = "1080x1920"
	}

	// 產生 ASS 檔案
	// 為了預覽，只需建立一個只包含該話的 segments
	segments := []SubtitleLine{
		{Start: 0, End: 5000, Text: text},
	}

	// 使用 BuildASS 產生 ASS 檔案
	assPath, _, err := BuildASS(tmpDir, style, segments, resolution)
	if err != nil {
		return "", err
	}
	defer os.Remove(assPath) // 確保刪除臨時字幕檔
	// 輸出圖片路徑
	outPath := filepath.Join(tmpDir, fmt.Sprintf("preview_%d.png", time.Now().UnixNano()))

	// 處理背景顏色
	if bgColor == "" {
		bgColor = "black@0.0" // 預設透明/黑色
	} else {
		// 如果有指定，需確保格式正確
		if !strings.HasPrefix(bgColor, "#") && len(bgColor) == 6 {
			bgColor = "0x" + bgColor // ffmpeg color syntax supports 0xRRGGBB
		} else if strings.HasPrefix(bgColor, "#") {
			bgColor = strings.Replace(bgColor, "#", "0x", 1)
		}
		// 加上不透明度 @1.0 (完全不透明)
		bgColor = bgColor + "@1.0"
	}

	// 使用 ffmpeg 產生預覽圖
	// -f lavfi -i color=c=... 產生背景
	// -vf "ass=filename.ass" 燒錄字幕
	// -frames:v 1 輸出第一幀

	vf := fmt.Sprintf("ass='%s'", strings.ReplaceAll(assPath, "\\", "/"))

	_, err = utils.RunCmd("ffmpeg", "-y",
		"-f", "lavfi", "-i", fmt.Sprintf("color=c=%s:s=%s:d=0.1", bgColor, resolution),
		"-vf", vf,
		"-frames:v", "1",
		"-f", "image2", outPath)

	if err != nil {
		return "", fmt.Errorf("ffmpeg 預覽失敗: %v", err)
	}

	return outPath, nil
}
