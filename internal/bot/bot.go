// SPDX-License-Identifier: Apache-2.0
package bot

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/FreshLabDev/makeitMD/internal/db"
	"github.com/FreshLabDev/makeitMD/internal/metrics"
	"github.com/FreshLabDev/makeitMD/internal/richmarkdown"
	"github.com/FreshLabDev/makeitMD/internal/telegram"
)

const (
	startText     = "Send me Markdown. I’ll render it."
	errorText     = "I couldn’t render that Markdown. Check the syntax and try again."
	chunkDebounce = 700 * time.Millisecond
	chunkMaxGap   = 2 * time.Second
)

type Store interface {
	Offset(context.Context) (int64, error)
	AdvanceOffset(context.Context, int64) error
	Touch(context.Context, telegram.User) error
	CreateConversion(context.Context, int64, telegram.Message, string) (int64, db.ConversionStatus, error)
	MarkSent(context.Context, int64, string, telegram.Result) error
	MarkFailed(context.Context, int64, string, telegram.Result) error
}

type Telegram interface {
	SetStartCommand(context.Context) error
	GetUpdates(context.Context, int64) ([]telegram.Update, error)
	SendText(context.Context, int64, string) error
	SendRichMarkdown(context.Context, int64, string) (telegram.Result, error)
}

type Bot struct {
	telegram     Telegram
	store        Store
	log          *slog.Logger
	lastPollUnix atomic.Int64
}

func New(client Telegram, data Store, log *slog.Logger) *Bot {
	return &Bot{telegram: client, store: data, log: log}
}

func (b *Bot) Run(ctx context.Context) error {
	if err := b.telegram.SetStartCommand(ctx); err != nil {
		b.log.Warn("set telegram commands failed", "error", err)
	}
	offset, err := b.store.Offset(ctx)
	if err != nil {
		return err
	}
	var pollFailures int
	var lastFailedUpdate int64
	var failedRetries int
	const maxUpdateRetries = 3
	for ctx.Err() == nil {
		updates, err := b.telegram.GetUpdates(ctx, offset)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			pollFailures++
			metrics.PollingErrors.Inc()
			delay := pollRetryDelay(pollFailures)
			if pollFailures <= 3 || pollFailures%10 == 0 {
				b.log.Error("telegram polling failed", "error", err, "consecutive_failures", pollFailures, "retry_in", delay)
			}
			if !wait(ctx, delay) {
				break
			}
			continue
		}
		if pollFailures > 0 {
			b.log.Info("telegram polling recovered", "after_failures", pollFailures)
			pollFailures = 0
		}
		if hasBatchableText(updates) && wait(ctx, chunkDebounce) {
			if refreshed, refreshErr := b.telegram.GetUpdates(ctx, offset); refreshErr == nil && len(refreshed) >= len(updates) {
				updates = refreshed
			}
		}
		b.lastPollUnix.Store(time.Now().Unix())
		for index := 0; index < len(updates); {
			update, lastIndex := groupTextUpdates(updates, index)
			handleErr := b.handle(ctx, update)
			if handleErr != nil {
				if ctx.Err() != nil {
					return nil
				}
				if update.UpdateID == lastFailedUpdate {
					failedRetries++
				} else {
					lastFailedUpdate = update.UpdateID
					failedRetries = 1
				}
				if failedRetries <= maxUpdateRetries {
					b.log.Error("telegram update failed; will retry", "update_id", update.UpdateID, "attempt", failedRetries, "error", handleErr)
					if !wait(ctx, jitterDuration(250*time.Millisecond)) {
						return nil
					}
					break
				}
				b.log.Error("telegram update permanently failed; dropping", "update_id", update.UpdateID, "attempts", failedRetries, "error", handleErr)
				lastFailedUpdate = 0
				failedRetries = 0
			} else if update.UpdateID == lastFailedUpdate {
				lastFailedUpdate = 0
				failedRetries = 0
			}
			offset = updates[lastIndex].UpdateID + 1
			if err := b.store.AdvanceOffset(ctx, offset); err != nil {
				return err
			}
			metrics.UpdatesProcessed.Inc()
			index = lastIndex + 1
		}
	}
	return nil
}

func (b *Bot) LastPoll() time.Time {
	unix := b.lastPollUnix.Load()
	if unix == 0 {
		return time.Time{}
	}
	return time.Unix(unix, 0)
}

func (b *Bot) handle(ctx context.Context, update telegram.Update) error {
	message := update.Message
	if message == nil || message.From == nil || message.From.IsBot || message.Chat.Type != "private" || message.Text == "" {
		return nil
	}
	if err := b.store.Touch(ctx, *message.From); err != nil {
		return err
	}
	command := strings.Fields(message.Text)
	if len(command) > 0 && command[0] == "/start" {
		return b.telegram.SendText(ctx, message.Chat.ID, startText)
	}
	if strings.HasPrefix(message.Text, "/") {
		return nil
	}
	renderedMarkdown := richmarkdown.RestoreEntities(message.Text, message.Entities)
	conversionID, status, err := b.store.CreateConversion(ctx, update.UpdateID, *message, renderedMarkdown)
	if err != nil {
		return err
	}
	switch status {
	case db.ConversionSent:
		return nil
	case db.ConversionFailed:
		return b.telegram.SendText(ctx, message.Chat.ID, errorText)
	}
	response, err := b.telegram.SendRichMarkdown(ctx, message.Chat.ID, renderedMarkdown)
	if err != nil {
		var apiErr *telegram.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusTooManyRequests {
			metrics.TelegramRateLimit.Inc()
		}
		if errors.As(err, &apiErr) && apiErr.ErrorCode == 400 {
			normalized := richmarkdown.NormalizeFallback(renderedMarkdown)
			if normalized != renderedMarkdown {
				retryResponse, retryErr := b.telegram.SendRichMarkdown(ctx, message.Chat.ID, normalized)
				if retryErr == nil {
					if markErr := b.store.MarkSent(ctx, conversionID, normalized, retryResponse); markErr != nil {
						return markErr
					}
					metrics.ConversionsNormalized.Inc()
					metrics.ConversionsSent.Inc()
					return nil
				} else if !isBadRequest(retryErr) {
					return retryErr
				} else {
					response = apiResponse(retryErr)
				}
			}
			if len(response) == 0 {
				response = apiResponse(err)
			}
			if markErr := b.store.MarkFailed(ctx, conversionID, "telegram_bad_request", response); markErr != nil {
				return markErr
			}
			metrics.ConversionsFailed.Inc()
			return b.telegram.SendText(ctx, message.Chat.ID, errorText)
		}
		return err
	}
	if err := b.store.MarkSent(ctx, conversionID, renderedMarkdown, response); err != nil {
		return err
	}
	metrics.ConversionsSent.Inc()
	return nil
}

func apiResponse(err error) telegram.Result {
	var apiErr *telegram.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Response
	}
	return nil
}

func isBadRequest(err error) bool {
	var apiErr *telegram.APIError
	return errors.As(err, &apiErr) && apiErr.ErrorCode == http.StatusBadRequest
}

func hasBatchableText(updates []telegram.Update) bool {
	for _, update := range updates {
		if batchableMessage(update.Message) {
			return true
		}
	}
	return false
}

func groupTextUpdates(updates []telegram.Update, start int) (telegram.Update, int) {
	first := updates[start]
	if !batchableMessage(first.Message) {
		return first, start
	}
	combined := *first.Message
	combined.Entities = append([]telegram.MessageEntity(nil), first.Message.Entities...)
	combined.RawMessages = append([]telegram.Result(nil), first.Message.RawMessages...)
	last := start
	for next := start + 1; next < len(updates); next++ {
		candidate := updates[next].Message
		previous := updates[last].Message
		if !samePaste(previous, candidate) {
			break
		}
		offset := utf16Length(combined.Text) + 1
		combined.Text += "\n" + candidate.Text
		combined.RawMessages = append(combined.RawMessages, candidate.RawMessages...)
		for _, entity := range candidate.Entities {
			entity.Offset += offset
			combined.Entities = append(combined.Entities, entity)
		}
		last = next
	}
	first.Message = &combined
	return first, last
}

func utf16Length(text string) int {
	length := 0
	for _, r := range text {
		if r > 0xffff {
			length += 2
		} else {
			length++
		}
	}
	return length
}

func batchableMessage(message *telegram.Message) bool {
	if message == nil || message.From == nil || message.From.IsBot || message.Chat.Type != "private" || message.Text == "" {
		return false
	}
	return !strings.HasPrefix(strings.TrimSpace(message.Text), "/")
}

func samePaste(previous, candidate *telegram.Message) bool {
	if !batchableMessage(previous) || !batchableMessage(candidate) {
		return false
	}
	if previous.Chat.ID != candidate.Chat.ID || previous.From.ID != candidate.From.ID || candidate.MessageID != previous.MessageID+1 {
		return false
	}
	if previous.Date == 0 || candidate.Date == 0 {
		return true
	}
	gap := time.Duration(candidate.Date-previous.Date) * time.Second
	return gap >= 0 && gap <= chunkMaxGap
}

func pollRetryDelay(failures int) time.Duration {
	if failures < 1 {
		failures = 1
	}
	if failures > 6 {
		failures = 6
	}
	delay := 2 * time.Second << (failures - 1)
	if delay > time.Minute {
		delay = time.Minute
	}
	return jitterDuration(delay)
}

func jitterDuration(duration time.Duration) time.Duration {
	if duration <= 0 {
		return duration
	}
	half := int64(duration / 2)
	return duration - time.Duration(half) + time.Duration(rand.Int63n(2*half+1))
}

func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
