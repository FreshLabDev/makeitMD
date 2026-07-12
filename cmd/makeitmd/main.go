// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/FreshLabDev/makeitMD/internal/bot"
	"github.com/FreshLabDev/makeitMD/internal/config"
	"github.com/FreshLabDev/makeitMD/internal/db"
	"github.com/FreshLabDev/makeitMD/internal/health"
	"github.com/FreshLabDev/makeitMD/internal/metrics"
	"github.com/FreshLabDev/makeitMD/internal/telegram"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := run(); err != nil {
		slog.Error("makeitMD stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	log.Info("makeitMD starting", "version", version, "commit", commit, "built", date)
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	data, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer data.Close()
	if cfg.AutoMigrate {
		if err := data.Migrate(ctx, cfg.MigrationsDir); err != nil {
			return err
		}
	}
	service := bot.New(telegram.NewClient(cfg.TelegramBotToken), data, log)
	startedAt := time.Now()
	mux := http.NewServeMux()
	mux.Handle("GET /healthz", health.New(data, service.LastPoll, startedAt, health.Build{Version: version, Commit: commit, Date: date}, log))
	mux.Handle("GET /metrics", metrics.Handler())
	server := &http.Server{
		Addr: cfg.HTTPAddr, Handler: mux,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second,
	}

	errCh := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Info("health server starting", "addr", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Info("telegram polling starting")
		if err := service.Run(ctx); err != nil {
			errCh <- err
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		runCleaner(ctx, data, cfg.ConversionRetention, log)
	}()

	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-errCh:
	}
	if runErr == nil {
		select {
		case runErr = <-errCh:
		default:
		}
	}
	stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	_ = server.Shutdown(shutdownCtx)
	cancel()
	wg.Wait()
	return runErr
}

func runCleaner(ctx context.Context, store *db.Store, retention time.Duration, log *slog.Logger) {
	clean := func() {
		deleted, err := store.CleanupConversions(ctx, retention)
		if err != nil && ctx.Err() == nil {
			log.Warn("conversion cleanup failed", "error", err)
		} else if deleted > 0 {
			log.Info("expired conversions deleted", "count", deleted)
		}
	}
	clean()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			clean()
		}
	}
}
