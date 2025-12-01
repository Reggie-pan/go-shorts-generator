package config

import (
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	Port        string
	StoragePath string
	AzureKey    string
	AzureRegion string
	FreeTTSPath string
	BgmPath     string
	GeminiKey   string
	AIModel     string
}

func Load() (*Config, error) {
	viper.SetDefault("PORT", "8080")
	viper.SetDefault("STORAGE_PATH", "/data")
	viper.SetDefault("BGM_PATH", "/assets/bgm")
	viper.SetDefault("AI_MODEL", "gemini-2.0-flash")

	viper.AutomaticEnv()

	cfg := &Config{
		Port:        viper.GetString("PORT"),
		StoragePath: viper.GetString("STORAGE_PATH"),
		AzureKey:    viper.GetString("AZURE_TTS_KEY"),
		AzureRegion: viper.GetString("AZURE_TTS_REGION"),
		FreeTTSPath: viper.GetString("FREE_TTS_MODEL_PATH"),
		BgmPath:     viper.GetString("BGM_PATH"),
		GeminiKey:   viper.GetString("GEMINI_API_KEY"),
		AIModel:     viper.GetString("AI_MODEL"),
	}

	if err := os.MkdirAll(cfg.StoragePath, 0o755); err != nil {
		return nil, err
	}
	return cfg, nil
}
