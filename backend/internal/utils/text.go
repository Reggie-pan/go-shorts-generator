package utils

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// AutoSpacing 為中文/英文之間加入空格，避免黏在一起
func AutoSpacing(s string) string {
	// 中文後接英文數字
	p1 := regexp.MustCompile(`([\p{Han}])([A-Za-z0-9])`)
	s = p1.ReplaceAllString(s, "$1 $2")
	// 英文數字接中文
	p2 := regexp.MustCompile(`([A-Za-z0-9])([\p{Han}])`)
	s = p2.ReplaceAllString(s, "$1 $2")
	return strings.TrimSpace(s)
}

// SplitScript 簡單斷句，如果太長以字數分段
func SplitScript(script string, maxLen int) []string {
	clean := strings.TrimSpace(script)
	if clean == "" {
		return []string{}
	}
	// 擴充分隔符號：包含頓號、分號
	delims := regexp.MustCompile(`[，。？！、；]`)
	parts := delims.Split(clean, -1)
	res := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// 如果片段過長，需要強制切分
		for utf8.RuneCountInString(p) > maxLen {
			runes := []rune(p)
			splitIdx := maxLen

			// 尋找最佳切分點，往回找 (最多回溯 8 個字元或一半長度)
			limit := 8
			if maxLen/2 < limit {
				limit = maxLen / 2
			}

			for k := 0; k < limit; k++ {
				idx := maxLen - k
				if idx <= 0 || idx >= len(runes) {
					continue
				}

				curr := runes[idx]
				prev := runes[idx-1]

				// 檢查是否為不安全切分點
				isUnsafe := false

				// 1. 英文/數字中間 (簡單判斷 ASCII)
				isPrevAlphanumeric := (prev >= 'a' && prev <= 'z') || (prev >= 'A' && prev <= 'Z') || (prev >= '0' && prev <= '9')
				isCurrAlphanumeric := (curr >= 'a' && curr <= 'z') || (curr >= 'A' && curr <= 'Z') || (curr >= '0' && curr <= '9')
				if isPrevAlphanumeric && isCurrAlphanumeric {
					isUnsafe = true
				}

				// 2. 特殊符號前 (%, ％, ℃, °)
				if curr == '%' || curr == '％' || curr == '℃' || curr == '°' {
					isUnsafe = true
				}

				// 3. 避頭點 (不應該出現在行首的符號)
				// 這裡簡單列舉常見的：，。？！、；：」』）
				if strings.ContainsRune("，。？！、；：」』）!,.:;?)]}", curr) {
					isUnsafe = true
				}

				if !isUnsafe {
					splitIdx = idx
					break
				}
			}

			res = append(res, strings.TrimSpace(string(runes[:splitIdx])))
			p = strings.TrimSpace(string(runes[splitIdx:]))
		}
		if p != "" {
			res = append(res, p)
		}
	}

	// 智能合併：如果連續兩個片段總長度 <= maxLen，則合併
	merged := make([]string, 0, len(res))
	i := 0
	for i < len(res) {
		current := res[i]
		// 嘗試與下一個片段合併
		if i+1 < len(res) {
			next := res[i+1]
			combined := current + next
			if utf8.RuneCountInString(combined) <= maxLen {
				merged = append(merged, combined)
				i += 2 // 跳過已合併的兩個片段
				continue
			}
		}
		merged = append(merged, current)
		i++
	}

	return merged
}

type SubtitleSegment struct {
	Text  string
	Start int // ms
	End   int // ms
}

// BuildTimeline 依 TTS 時長與字幕行，計算時間軸，並校正總時長
func BuildTimeline(lines []string, durations []int) []SubtitleSegment {
	segments := []SubtitleSegment{}
	var cursor int
	for i, line := range lines {
		dur := 0
		if i < len(durations) {
			dur = durations[i]
		}
		if dur == 0 {
			dur = 1000
		}
		seg := SubtitleSegment{
			Text:  line,
			Start: cursor,
			End:   cursor + dur,
		}
		segments = append(segments, seg)
		cursor += dur
	}
	return segments
}

// BuildTimelineFloat 依 TTS 時長 (秒) 與字幕行，計算時間軸 (ms)，使用 float64 減少累積誤差
func BuildTimelineFloat(lines []string, durations []float64) []SubtitleSegment {
	segments := []SubtitleSegment{}
	var cursor float64
	for i, line := range lines {
		dur := 0.0
		if i < len(durations) {
			dur = durations[i]
		}
		if dur == 0 {
			dur = 1.0
		}

		startMs := int(cursor * 1000)
		endMs := int((cursor + dur) * 1000)

		seg := SubtitleSegment{
			Text:  line,
			Start: startMs,
			End:   endMs,
		}
		segments = append(segments, seg)
		cursor += dur
	}
	return segments
}
