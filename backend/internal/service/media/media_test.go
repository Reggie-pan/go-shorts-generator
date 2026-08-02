package media

import (
	"testing"
)

func TestWrapText_Chinese(t *testing.T) {
	text := "半導體測試設備龍頭鴻勁傳出大消息"
	maxLineWidth := 10.0
	got := wrapText(text, maxLineWidth)
	// 16 個中文字，雙行長度平衡：半導體測試設備龍 (8字) \n 頭鴻勁傳出大消息 (8字)
	expected := "半導體測試設備龍\n頭鴻勁傳出大消息"
	if got != expected {
		t.Errorf("expected:\n%q\ngot:\n%q", expected, got)
	}
}

func TestWrapText_EnglishWordProtection(t *testing.T) {
	text := "The quick brown fox jumps over the lazy dog"
	maxLineWidth := 10.0
	got := wrapText(text, maxLineWidth)
	// 驗證英文單字 jumps/brown 等保持完整不被切斷
	expected := "The quick brown fox\njumps over the lazy dog"
	if got != expected {
		t.Errorf("expected:\n%q\ngot:\n%q", expected, got)
	}
}

func TestWrapText_MixedSpace(t *testing.T) {
	text := "每月交付量上看 30 台"
	maxLineWidth := 10.0
	got := wrapText(text, maxLineWidth)
	// 加權總寬度 10.0 <= 10.0，完美單行呈現不換行
	expected := "每月交付量上看 30 台"
	if got != expected {
		t.Errorf("expected:\n%q\ngot:\n%q", expected, got)
	}
}

func TestWrapText_Insertion(t *testing.T) {
	text := "Insertion 4 OE 光電同測設備"
	maxLineWidth := 10.0
	got := wrapText(text, maxLineWidth)
	// 雙行長度平衡：Insertion 4 OE \n 光電同測設備
	expected := "Insertion 4 OE\n光電同測設備"
	if got != expected {
		t.Errorf("expected:\n%q\ngot:\n%q", expected, got)
	}
}

func TestWrapText_CPOSentence(t *testing.T) {
	text := "你認為 CPO 會成為明年"
	// 1080p, size 90, 0.90 比例 -> maxLineWidth = 10.8
	maxLineWidth := 10.8
	got := wrapText(text, maxLineWidth)
	// 加權總寬度 = 3.0(中文) + 0.25(空格) + 1.2(CPO) + 0.25(空格) + 5.0(中文) = 9.7 <= 10.8
	// 期望 100% 保持單行，不觸發換行
	expected := "你認為 CPO 會成為明年"
	if got != expected {
		t.Errorf("expected:\n%q\ngot:\n%q", expected, got)
	}
}
