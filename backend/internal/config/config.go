package config

import (
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	Port        string
	StoragePath string
	GoogleKey   string
	AzureKey    string
	AzureRegion string
	FreeTTSPath string
	BgmPath     string
}

func Load() (*Config, error) {
	viper.SetDefault("PORT", "8080")
	viper.SetDefault("STORAGE_PATH", "/data")
	viper.SetDefault("BGM_PATH", "/assets/bgm")

	viper.AutomaticEnv()

	cfg := &Config{
		Port:        viper.GetString("PORT"),
		StoragePath: viper.GetString("STORAGE_PATH"),
		GoogleKey:   viper.GetString("GOOGLE_TTS_KEY"),
		AzureKey:    viper.GetString("AZURE_TTS_KEY"),
		AzureRegion: viper.GetString("AZURE_TTS_REGION"),
		FreeTTSPath: viper.GetString("FREE_TTS_MODEL_PATH"),
		BgmPath:     viper.GetString("BGM_PATH"),
	}

	if err := os.MkdirAll(cfg.StoragePath, 0o755); err != nil {
		return nil, err
	}
	return cfg, nil
}
