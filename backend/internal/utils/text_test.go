package utils

import "testing"

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
