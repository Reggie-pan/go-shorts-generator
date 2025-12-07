package tts

import (
	"fmt"

	"github.com/Reggie-pan/go-shorts-generator/internal/config"
)

type Voice struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Locale      string `json:"locale"`
	Gender      string `json:"gender"`
}

type Provider interface {
	Synthesize(text, voice, locale string, speed, pitch float64) (string, float64, error)
	ListVoices() ([]Voice, error)
}

func GetProvider(name string, cfg *config.Config) (Provider, error) {
	switch name {
	case "azure_v1", "azure_v2":
		return &AzureProvider{Key: cfg.AzureKey, Region: cfg.AzureRegion}, nil
	case "edge_tts":
		return &EdgeTTSProvider{}, nil
	default:
		return nil, fmt.Errorf("未知的 TTS provider: %s", name)
	}
}
