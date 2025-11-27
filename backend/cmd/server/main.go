package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"video-smith/backend/internal/ai"
	"video-smith/backend/internal/api"
	"video-smith/backend/internal/config"
	"video-smith/backend/internal/storage"
	"video-smith/backend/internal/worker"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("讀取設定失敗")
	}

	if cfg.GeminiKey == "" {
		log.Fatal().Msg("GEMINI_API_KEY is missing")
	}

	db, err := storage.NewStore(cfg.StoragePath)
	if err != nil {
		log.Fatal().Err(err).Msg("初始化儲存失敗")
	}
	defer db.Close()

	aiClient, err := ai.NewClient(cfg.GeminiKey, cfg.AIModel)
	if err != nil {
		log.Fatal().Err(err).Msg("初始化 AI 失敗")
	}
	defer aiClient.Close()

	jobQueue := worker.NewQueue(10)
	w := worker.NewWorker(cfg, db, jobQueue, aiClient)
	go w.Run()

	r := api.NewRouter(cfg, db, jobQueue)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		log.Info().Msgf("伺服器啟動於 :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("伺服器錯誤")
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Info().Msg("收到中斷訊號，關閉中...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("關閉伺服器失敗")
	}
}
