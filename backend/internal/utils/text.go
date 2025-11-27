package utils

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// AutoSpacing 於中英文/數字界線插入空格，避免黏在一起。
func AutoSpacing(s string) string {
	// 中文後接英文或數字
	p1 := regexp.MustCompile(`([\p{Han}])([A-Za-z0-9])`)
	s = p1.ReplaceAllString(s, "$1 $2")
	// 英文或數字後接中文
	p2 := regexp.MustCompile(`([A-Za-z0-9])([\p{Han}])`)
	s = p2.ReplaceAllString(s, "$1 $2")
	return strings.TrimSpace(s)
}

// SplitScript 按標點切句，如仍過長則以字數分段。
func SplitScript(script string, maxLen int) []string {
	clean := strings.TrimSpace(script)
	if clean == "" {
		return []string{}
	}
	delims := regexp.MustCompile(`[。！？!?；;]`)
	parts := delims.Split(clean, -1)
	res := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		for utf8.RuneCountInString(p) > maxLen {
			runes := []rune(p)
			res = append(res, strings.TrimSpace(string(runes[:maxLen])))
			p = strings.TrimSpace(string(runes[maxLen:]))
		}
		if p != "" {
			res = append(res, p)
		}
	}
	return res
}

type SubtitleSegment struct {
	Text  string
	Start int // ms
	End   int // ms
}

// BuildTimeline 依 TTS 句長生成字幕時間軸，並校正為連續。
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

// BuildTimelineFloat 依 TTS 句長 (秒) 生成字幕時間軸 (ms)，使用 float64 避免累積誤差。
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
