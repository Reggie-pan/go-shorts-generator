package utils

import (
	"strings"
	"testing"
)

func TestAutoSpacing(t *testing.T) {
	got := AutoSpacing("AI工具123")
	want := "AI 工具 123"
	if got != want {
		t.Fatalf("期望 %s 得到 %s", want, got)
	}
}

func TestSplitScript(t *testing.T) {
	lines := SplitScript("這是一段測試腳本，中文內容需要被斷句。第二句測試!", 8)
	if len(lines) < 2 {
		t.Fatalf("應至少分兩句，得到 %v", lines)
	}
	for _, l := range lines {
		if len([]rune(l)) > 8 {
			t.Fatalf("斷句過長: %s", l)
		}
	}
}

func TestBuildTimeline(t *testing.T) {
	lines := []string{"a", "b"}
	durs := []int{1000, 2000}
	segs := BuildTimeline(lines, durs)
	if segs[0].Start != 0 || segs[0].End != 1000 {
		t.Fatalf("第一段時間錯誤 %+v", segs[0])
	}
	if segs[1].Start != 1000 || segs[1].End != 3000 {
		t.Fatalf("第二段時間錯誤 %+v", segs[1])
	}
}

func TestSplitScriptSmart(t *testing.T) {
	// Case 1: Number and Unit (35％)
	// 總長 17，MaxLen 16
	// 期望切分點避開 35 和 ％ 之間
	text := "外資預估2026年營收將年增35％。"
	lines := SplitScript(text, 16)

	found := false
	for _, l := range lines {
		if strings.Contains(l, "35％") {
			found = true
		}
	}
	if !found {
		t.Errorf("應該包含完整 '35％', 但得到: %v", lines)
	}

	// Case 2: English Words with spaces
	text2 := "Hello World Test"
	lines2 := SplitScript(text2, 5)
	if len(lines2) != 3 {
		t.Errorf("Expect 3 lines for '%s', got %d: %v", text2, len(lines2), lines2)
	}
	if lines2[0] != "Hello" || lines2[1] != "World" || lines2[2] != "Test" {
		t.Errorf("Unexpected split result: %v", lines2)
	}

	// Case 3: Avoid splitting English word
	// "This is a test" -> "This is a" (9 chars)
	// MaxLen 6.
	// "This i" (6). "s" | " ". Safe.
	// "This i" -> "s" is alpha, " " is not.
	// Wait, "This i" -> "i" | "s". Unsafe.
	// "This " -> " " | "i". Safe.
	// So it should split at "This ".
	text3 := "This is a test"
	lines3 := SplitScript(text3, 6)
	// Expect "This", "is a", "test" ?
	// "This " (5).
	// "is a t" (6). "is a " (5).
	// "test"

	if lines3[0] != "This" {
		t.Errorf("Expected 'This', got '%s'", lines3[0])
	}
}
