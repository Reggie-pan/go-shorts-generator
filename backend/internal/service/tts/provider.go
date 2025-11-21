package tts

import (
	"fmt"

	"video-smith/backend/internal/config"
)

type Provider interface {
	Synthesize(text, voice, locale string, speed, pitch float64) (string, int, error)
}

func GetProvider(name string, cfg *config.Config) (Provider, error) {
	switch name {
	case "google":
		return &ShellProvider{Engine: "google", Key: cfg.GoogleKey}, nil
	case "azure_v1", "azure_v2":
		return &AzureProvider{Key: cfg.AzureKey, Region: cfg.AzureRegion}, nil
	case "free_espeak", "free":
		return &EspeakProvider{}, nil
	default:
		return nil, fmt.Errorf("未知的 TTS provider: %s", name)
	}
}
