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

	"github.com/Reggie-pan/go-shorts-generator/internal/ai"
	"github.com/Reggie-pan/go-shorts-generator/internal/api"
	"github.com/Reggie-pan/go-shorts-generator/internal/config"
	"github.com/Reggie-pan/go-shorts-generator/internal/service/job"
	"github.com/Reggie-pan/go-shorts-generator/internal/storage"
	"github.com/Reggie-pan/go-shorts-generator/internal/worker"
)

func main() {
	// 使用 ConsoleWriter 輸出人性化的文字日誌，便於開發與手動排查
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

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

	// D2: 巡檢並重置伺服器重啟前處於 running 狀態的 Job
	if records, _, err := db.ListJobs(1, 100000); err == nil {
		for _, rec := range records {
			if rec.Status == job.StatusRunning {
				rec.Status = job.StatusFailed
				rec.ErrorMessage = "伺服器重啟中斷"
				rec.Progress = 0
				rec.UpdatedAt = time.Now()
				if err := db.UpdateJob(rec); err != nil {
					log.Error().Err(err).Str("job", rec.ID).Msg("啟動時重設任務狀態失敗")
				} else {
					log.Info().Str("job", rec.ID).Msg("已成功重設中斷任務為 Failed 狀態")
				}
			}
		}
	}

	aiClient, err := ai.NewClient(cfg.GeminiKey, cfg.AIModel)
	if err != nil {
		log.Fatal().Err(err).Msg("初始化 AI 失敗")
	}
	defer aiClient.Close()

	jobQueue := worker.NewQueue(10)
	w := worker.NewWorker(cfg, db, jobQueue, aiClient)
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	go w.Run(workerCtx)

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

	// 1. 立即通知 Worker 關閉並中斷運行中的外部 FFmpeg (D1)
	cancelWorker()

	// 2. 最多優雅等待當前任務 10 秒 (D1)
	w.Wait(10 * time.Second)

	// 3. 關閉 HTTP 伺服器
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("關閉伺服器失敗")
	}
}
