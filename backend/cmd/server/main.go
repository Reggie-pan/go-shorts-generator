package main

import (
	"context"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/Reggie-pan/go-shorts-generator/internal/ai"
	"github.com/Reggie-pan/go-shorts-generator/internal/api"
	"github.com/Reggie-pan/go-shorts-generator/internal/config"
	"github.com/Reggie-pan/go-shorts-generator/internal/storage"
	"github.com/Reggie-pan/go-shorts-generator/internal/worker"
)

func main() {
	// 使用 ConsoleWriter 輸出以符合使用者需求，並嘗試寫入 Stderr 避免 stdout 緩衝問題
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	// 關閉預設 JSON 輸出以獲取清晰的 log 輸出，並啟用 ConsoleWriter 可能的緩衝修正

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
