// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

type Counter struct{ value atomic.Int64 }

func (c *Counter) Inc()         { c.value.Add(1) }
func (c *Counter) Value() int64 { return c.value.Load() }

var (
	PollingErrors         Counter
	UpdatesProcessed      Counter
	ConversionsSent       Counter
	ConversionsFailed     Counter
	ConversionsNormalized Counter
	TelegramRateLimit     Counter
)

func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		writeCounter(w, "makeitmd_polling_errors_total", "Telegram polling failures.", PollingErrors.Value())
		writeCounter(w, "makeitmd_updates_processed_total", "Telegram updates acknowledged.", UpdatesProcessed.Value())
		writeCounter(w, "makeitmd_conversions_sent_total", "Rich Markdown messages sent successfully.", ConversionsSent.Value())
		writeCounter(w, "makeitmd_conversions_failed_total", "Rich Markdown inputs rejected by Telegram.", ConversionsFailed.Value())
		writeCounter(w, "makeitmd_conversions_normalized_total", "Messages delivered through the GitHub HTML compatibility fallback.", ConversionsNormalized.Value())
		writeCounter(w, "makeitmd_telegram_rate_limits_total", "Telegram requests still rate-limited after retry.", TelegramRateLimit.Value())
	})
}

func writeCounter(w http.ResponseWriter, name, help string, value int64) {
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n%s %d\n", name, help, name, name, value)
}
