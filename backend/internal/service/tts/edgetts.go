package tts

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Reggie-pan/go-shorts-generator/internal/utils"
	"github.com/wujunwei928/edge-tts-go/edge_tts"
)

// EdgeTTSProvider 實作 Microsoft Edge 免費線上 TTS 服務
type EdgeTTSProvider struct{}

func (e *EdgeTTSProvider) Synthesize(text, voice, locale string, speed, pitch float64) (string, float64, error) {
	if voice == "" {
		// 預設使用台灣中文語音
		voice = "zh-TW-HsiaoChenNeural"
	}

	// 將 speed (1.0 為正常速度) 轉換為 Edge TTS 的格式 (e.g. "+0%", "+50%", "-25%")
	ratePercent := (speed - 1.0) * 100
	rateStr := fmt.Sprintf("%+.0f%%", ratePercent)

	// 將 pitch 轉換為 Edge TTS 的格式 (e.g. "+0Hz", "+50Hz", "-50Hz")
	pitchHz := pitch * 50 // 將百分比轉換為 Hz 範圍
	pitchStr := fmt.Sprintf("%+.0fHz", pitchHz)

	// 建立 Communicate 實例
	c, err := edge_tts.NewCommunicate(text,
		edge_tts.SetVoice(voice),
		edge_tts.SetRate(rateStr),
		edge_tts.SetPitch(pitchStr),
	)
	if err != nil {
		return "", 0, fmt.Errorf("Edge TTS 初始化失敗: %w", err)
	}

	// 合成音訊
	data, err := c.Stream()
	if err != nil {
		return "", 0, fmt.Errorf("Edge TTS 合成失敗: %w", err)
	}

	if len(data) < 100 {
		return "", 0, fmt.Errorf("Edge TTS 回傳資料太小 (%d bytes)", len(data))
	}

	// 寫入暫存檔案 (Edge TTS 輸出為 MP3 格式)
	tmp := filepath.Join(os.TempDir(), "edge_tts_"+strconv.FormatInt(time.Now().UnixNano(), 10)+".mp3")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", 0, fmt.Errorf("Edge TTS 寫入檔案失敗: %w", err)
	}

	dur, _ := utils.AudioDurationSeconds(tmp)
	return tmp, dur, nil
}

func (e *EdgeTTSProvider) ListVoices() ([]Voice, error) {
	edgeVoices, err := edge_tts.ListVoices("")
	if err != nil {
		return nil, fmt.Errorf("Edge TTS 取得語音列表失敗: %w", err)
	}

	var voices []Voice
	for _, v := range edgeVoices {
		// 從 ShortName 解析 Locale (格式如 "zh-TW-HsiaoChenNeural")
		parts := strings.Split(v.ShortName, "-")
		locale := ""
		if len(parts) >= 2 {
			locale = parts[0] + "-" + parts[1]
		}

		voices = append(voices, Voice{
			Name:        v.ShortName,
			DisplayName: fmt.Sprintf("%s (%s, %s)", v.FriendlyName, v.Locale, v.Gender),
			Locale:      locale,
			Gender:      v.Gender,
		})
	}

	return voices, nil
}
