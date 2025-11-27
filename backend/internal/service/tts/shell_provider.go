package tts

// ShellProvider 簡化：目前轉呼叫 espeak，並保留擴充位。
type ShellProvider struct {
	Engine string
	Key    string
	Region string
}

func (s *ShellProvider) Synthesize(text, voice, locale string, speed, pitch float64) (string, float64, error) {
	if s.Engine == "google" && s.Key == "" {
		return (&EspeakProvider{}).Synthesize(text, voice, locale, speed, pitch)
	}
	if s.Engine == "azure" && s.Key == "" {
		return (&EspeakProvider{}).Synthesize(text, voice, locale, speed, pitch)
	}
	// TODO: 可接官方 API，此處單元環境預設降級到 espeak。
	return (&EspeakProvider{}).Synthesize(text, voice, locale, speed, pitch)
}
