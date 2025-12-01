package worker

import (
	"testing"

	"github.com/Reggie-pan/go-shorts-generator/internal/utils"
)

func TestTimelineSync(t *testing.T) {
	// 模擬各片段長度 (秒)
	// 假設每段都已經包含靜音 (例如 0.2s)
	durations := []float64{
		2.7, // 第一句 (2.5 + 0.2)
		3.3, // 第二句 (3.1 + 0.2)
		2.0, // 第三句 (1.8 + 0.2)
	}

	lines := []string{"Line 1", "Line 2", "Line 3"}

	// 呼叫時間軸計算
	subs := utils.BuildTimelineFloat(lines, durations)

	// 驗證
	if len(subs) != len(lines) {
		t.Fatalf("Expected %d segments, got %d", len(lines), len(subs))
	}

	// 驗證總長度
	expectedTotalMs := int((2.7 + 3.3 + 2.0) * 1000)
	lastSub := subs[len(subs)-1]

	// 容許誤差 10ms (因為 float -> int 轉換)
	if abs(lastSub.End-expectedTotalMs) > 10 {
		t.Errorf("Total duration mismatch. Expected %d, got %d", expectedTotalMs, lastSub.End)
	}

	// 驗證連續性
	for i := 0; i < len(subs); i++ {
		expectedStart := 0
		if i > 0 {
			expectedStart = subs[i-1].End
		}

		if abs(subs[i].Start-expectedStart) > 1 {
			t.Errorf("Segment %d start mismatch. Expected %d, got %d", i, expectedStart, subs[i].Start)
		}

		expectedDur := int(durations[i] * 1000)
		actualDur := subs[i].End - subs[i].Start
		if abs(actualDur-expectedDur) > 10 {
			t.Errorf("Segment %d duration mismatch. Expected %d, got %d", i, expectedDur, actualDur)
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
