// SPDX-License-Identifier: Apache-2.0
package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("DATABASE_URL", "postgres://example/core")
	for _, key := range []string{"AUTO_MIGRATE", "HTTP_ADDR", "MIGRATIONS_DIR", "CONVERSION_RETENTION"} {
		t.Setenv(key, "")
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.AutoMigrate || cfg.HTTPAddr != ":8080" || cfg.ConversionRetention != 90*24*time.Hour {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadRejectsInvalidRetention(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("DATABASE_URL", "postgres://example/core")
	t.Setenv("CONVERSION_RETENTION", "forever")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid retention error")
	}
}
