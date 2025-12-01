package tts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Reggie-pan/go-shorts-generator/internal/utils"
)

// AzureProvider 實作 Azure 語音 REST API（v1/v2 通用）
type AzureProvider struct {
	Key    string
	Region string
}

func (a *AzureProvider) Synthesize(text, voice, locale string, speed, pitch float64) (string, float64, error) {
	if a.Key == "" || a.Region == "" {
		return "", 0, fmt.Errorf("Azure TTS key or region is missing")
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

	info, _ := f.Stat()
	if info.Size() < 100 {
		// 檔案太小，可能是空檔或只有檔頭
		content, _ := os.ReadFile(tmp)
		return "", 0, fmt.Errorf("Azure TTS 回傳檔案太小 (%d bytes), 內容: %s", info.Size(), string(content))
	}

	dur, _ := utils.AudioDurationSeconds(tmp)
	return tmp, dur, nil
}

func (a *AzureProvider) ListVoices() ([]Voice, error) {
	if a.Key == "" || a.Region == "" {
		return nil, fmt.Errorf("Azure TTS key or region is missing")
	}
	url := fmt.Sprintf("https://%s.tts.speech.microsoft.com/cognitiveservices/voices/list", a.Region)
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Ocp-Apim-Subscription-Key", a.Key)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list voices: %s", resp.Status)
	}

	// Azure 返回 JSON 結構
	type azureVoice struct {
		ShortName   string `json:"ShortName"`
		DisplayName string `json:"DisplayName"` // 這裡其實是 LocalName + (Description)
		LocalName   string `json:"LocalName"`
		Locale      string `json:"Locale"`
		Gender      string `json:"Gender"`
	}
	var raw []azureVoice
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	var voices []Voice
	for _, v := range raw {
		voices = append(voices, Voice{
			Name:        v.ShortName,
			DisplayName: fmt.Sprintf("%s (%s, %s)", v.LocalName, v.Locale, v.Gender),
			Locale:      v.Locale,
			Gender:      v.Gender,
		})
	}
	return voices, nil
}
