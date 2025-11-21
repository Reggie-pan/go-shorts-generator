package tts

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"video-smith/backend/internal/utils"
)

// AzureProvider 透過 Azure 語音 REST API（v1/v2 共用）。
type AzureProvider struct {
	Key    string
	Region string
}

func (a *AzureProvider) Synthesize(text, voice, locale string, speed, pitch float64) (string, int, error) {
	if a.Key == "" || a.Region == "" {
		return (&EspeakProvider{}).Synthesize(text, voice, locale, speed, pitch)
	}
	if voice == "" {
		voice = locale + "-AriaNeural"
	}
	rate := fmt.Sprintf("%+.0f%%", (speed-1.0)*100)
	pitchStr := fmt.Sprintf("%+.0f%%", pitch*100)
	ssml := fmt.Sprintf(`<speak version='1.0' xml:lang='%s'><voice name='%s'><prosody rate='%s' pitch='%s'>%s</prosody></voice></speak>`,
		locale, voice, rate, pitchStr, text)

	url := fmt.Sprintf("https://%s.tts.speech.microsoft.com/cognitiveservices/v1", a.Region)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(ssml))
	req.Header.Set("Content-Type", "application/ssml+xml")
	req.Header.Set("Ocp-Apim-Subscription-Key", a.Key)
	req.Header.Set("X-Microsoft-OutputFormat", "riff-24khz-16bit-mono-pcm")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("azure tts 錯誤: %s", string(body))
	}
	tmp := filepath.Join(os.TempDir(), "azure_tts_"+strconv.FormatInt(time.Now().UnixNano(), 10)+".wav")
	f, err := os.Create(tmp)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", 0, err
	}
	dur, _ := utils.AudioDurationMS(tmp)
	return tmp, dur, nil
}
