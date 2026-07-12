// SPDX-License-Identifier: Apache-2.0
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	TelegramBotToken    string
	DatabaseURL         string
	MigrationsDir       string
	AutoMigrate         bool
	HTTPAddr            string
	ConversionRetention time.Duration
	LogLevel            string
}

func Load() (Config, error) {
	cfg := Config{
		TelegramBotToken: strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		DatabaseURL:      strings.TrimSpace(os.Getenv("DATABASE_URL")),
		MigrationsDir:    valueOrDefault("MIGRATIONS_DIR", "./migrations"),
		HTTPAddr:         valueOrDefault("HTTP_ADDR", ":8080"),
		LogLevel:         valueOrDefault("LOG_LEVEL", "info"),
	}
	var err error
	if cfg.AutoMigrate, err = strconv.ParseBool(valueOrDefault("AUTO_MIGRATE", "true")); err != nil {
		return Config{}, fmt.Errorf("AUTO_MIGRATE must be a boolean: %w", err)
	}
	if cfg.ConversionRetention, err = time.ParseDuration(valueOrDefault("CONVERSION_RETENTION", "2160h")); err != nil || cfg.ConversionRetention <= 0 {
		return Config{}, fmt.Errorf("CONVERSION_RETENTION must be a positive duration")
	}
	if cfg.TelegramBotToken == "" {
		return Config{}, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

func valueOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
