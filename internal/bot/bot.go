// SPDX-License-Identifier: Apache-2.0
package bot

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math/rand"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/FreshLabDev/tg"

	"github.com/FreshLabDev/makeitMD/internal/db"
	"github.com/FreshLabDev/makeitMD/internal/metrics"
	"github.com/FreshLabDev/makeitMD/internal/richmarkdown"
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
	Touch(context.Context, tg.User) error
	CreateConversion(context.Context, int64, tg.Message, []json.RawMessage, string) (int64, db.ConversionStatus, error)
	MarkSent(context.Context, int64, string, db.Result, []db.DeliveryAttempt) error
	MarkFailed(context.Context, int64, string, string, db.Result, []db.DeliveryAttempt) error
}

// Telegram is the part of the shared client makeitMD uses. It is satisfied by
// *tg.Client as it stands.
type Telegram interface {
	SetMyCommands(context.Context, []tg.BotCommand) error
	GetUpdates(context.Context, int64, int) ([]tg.Update, error)
	SendMessage(context.Context, int64, string, *tg.InlineKeyboardMarkup) (tg.Message, error)
	SendRichMarkdown(context.Context, int64, string, *tg.InlineKeyboardMarkup, ...tg.RichOption) (tg.Message, error)
}

// pollTimeout is the long-poll duration in seconds. The client derives its own
// HTTP deadline from it.
const pollTimeout = 50

// startCommand is published once at startup.
var startCommand = []tg.BotCommand{{Command: "start", Description: "Start the bot"}}

// paste is what a person actually sent. Telegram splits a long paste into
// several messages, so one conversion can span more than one of them, and raws
// keeps each original exactly as it arrived for the audit trail.
type paste struct {
	updateID int64
	message  *tg.Message
	raws     []json.RawMessage
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
	if err := b.telegram.SetMyCommands(ctx, startCommand); err != nil {
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
		updates, err := b.telegram.GetUpdates(ctx, offset, pollTimeout)
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
			if refreshed, refreshErr := b.telegram.GetUpdates(ctx, offset, pollTimeout); refreshErr == nil && len(refreshed) >= len(updates) {
				updates = refreshed
			}
		}
		b.lastPollUnix.Store(time.Now().Unix())
		for index := 0; index < len(updates); {
			grouped, lastIndex := groupTextUpdates(updates, index)
			handleErr := b.handle(ctx, grouped)
			if handleErr != nil {
				if ctx.Err() != nil {
					return nil
				}
				if grouped.updateID == lastFailedUpdate {
					failedRetries++
				} else {
					lastFailedUpdate = grouped.updateID
					failedRetries = 1
				}
				if failedRetries <= maxUpdateRetries {
					b.log.Error("telegram update failed; will retry", "update_id", grouped.updateID, "attempt", failedRetries, "error", handleErr)
					if !wait(ctx, jitterDuration(250*time.Millisecond)) {
						return nil
					}
					break
				}
				b.log.Error("telegram update permanently failed; dropping", "update_id", grouped.updateID, "attempts", failedRetries, "error", handleErr)
				lastFailedUpdate = 0
				failedRetries = 0
			} else if grouped.updateID == lastFailedUpdate {
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

func (b *Bot) handle(ctx context.Context, p paste) error {
	message := p.message
	if message == nil || message.From == nil || message.From.IsBot || message.Chat.Type != "private" || message.Text == "" {
		return nil
	}
	if err := b.store.Touch(ctx, *message.From); err != nil {
		return err
	}
	command := strings.Fields(message.Text)
	if len(command) > 0 && command[0] == "/start" {
		return b.sendText(ctx, message.Chat.ID, startText)
	}
	if strings.HasPrefix(message.Text, "/") {
		return nil
	}
	renderedMarkdown := richmarkdown.RestoreEntities(message.Text, message.Entities)
	conversionID, status, err := b.store.CreateConversion(ctx, p.updateID, *message, p.raws, renderedMarkdown)
	if err != nil {
		return err
	}
	switch status {
	case db.ConversionSent:
		return nil
	case db.ConversionFailed:
		return b.sendText(ctx, message.Chat.ID, errorText)
	}
	response, err := b.sendRichMarkdown(ctx, message.Chat.ID, renderedMarkdown)
	attempts := []db.DeliveryAttempt{deliveryAttempt(renderedMarkdown, response, err)}
	if err != nil {
		var apiErr *tg.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusTooManyRequests {
			metrics.TelegramRateLimit.Inc()
		}
		if errors.As(err, &apiErr) && apiErr.ErrorCode == 400 {
			normalized := richmarkdown.NormalizeFallback(renderedMarkdown)
			if normalized != renderedMarkdown {
				retryResponse, retryErr := b.sendRichMarkdown(ctx, message.Chat.ID, normalized)
				attempts = append(attempts, deliveryAttempt(normalized, retryResponse, retryErr))
				if retryErr == nil {
					if markErr := b.store.MarkSent(ctx, conversionID, normalized, retryResponse, attempts); markErr != nil {
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
			if markErr := b.store.MarkFailed(ctx, conversionID, "telegram_bad_request", attempts[len(attempts)-1].Markdown, response, attempts); markErr != nil {
				return markErr
			}
			metrics.ConversionsFailed.Inc()
			return b.sendText(ctx, message.Chat.ID, errorText)
		}
		return err
	}
	if err := b.store.MarkSent(ctx, conversionID, renderedMarkdown, response, attempts); err != nil {
		return err
	}
	metrics.ConversionsSent.Inc()
	return nil
}

// sendText delivers one of this bot's own fixed strings. They are sent as
// HTML, so they must stay free of HTML metacharacters -- see startText and
// errorText, which are the only two.
func (b *Bot) sendText(ctx context.Context, chatID int64, text string) error {
	_, err := b.telegram.SendMessage(ctx, chatID, text, nil)
	return err
}

// sendRichMarkdown renders a paste and returns Telegram's answer exactly as it
// arrived, which is what the conversion record keeps. Entity detection stays
// on: this bot renders Markdown a person wrote, where a bare URL is meant to
// become a link.
func (b *Bot) sendRichMarkdown(ctx context.Context, chatID int64, markdown string) (db.Result, error) {
	message, err := b.telegram.SendRichMarkdown(ctx, chatID, markdown, nil, tg.WithEntityDetection())
	return message.Raw, err
}

func deliveryAttempt(markdown string, response db.Result, err error) db.DeliveryAttempt {
	attempt := db.DeliveryAttempt{Markdown: markdown, Response: response}
	if err != nil {
		attempt.Error = err.Error()
		if len(attempt.Response) == 0 {
			attempt.Response = apiResponse(err)
		}
	}
	return attempt
}

func apiResponse(err error) db.Result {
	var apiErr *tg.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Response
	}
	return nil
}

func isBadRequest(err error) bool {
	var apiErr *tg.APIError
	return errors.As(err, &apiErr) && apiErr.ErrorCode == http.StatusBadRequest
}

func hasBatchableText(updates []tg.Update) bool {
	for _, update := range updates {
		if batchableMessage(update.Message) {
			return true
		}
	}
	return false
}

func groupTextUpdates(updates []tg.Update, start int) (paste, int) {
	first := updates[start]
	if !batchableMessage(first.Message) {
		return paste{updateID: first.UpdateID, message: first.Message, raws: rawsOf(first.Message)}, start
	}
	combined := *first.Message
	combined.Entities = append([]tg.MessageEntity(nil), first.Message.Entities...)
	raws := rawsOf(first.Message)
	last := start
	for next := start + 1; next < len(updates); next++ {
		candidate := updates[next].Message
		previous := updates[last].Message
		if !samePaste(previous, candidate) {
			break
		}
		offset := utf16Length(combined.Text) + 1
		combined.Text += "\n" + candidate.Text
		raws = append(raws, rawsOf(candidate)...)
		for _, entity := range candidate.Entities {
			entity.Offset += offset
			combined.Entities = append(combined.Entities, entity)
		}
		last = next
	}
	// The stitched message is synthetic: it has no original of its own, so its
	// Raw would describe only the first part.
	combined.Raw = nil
	return paste{updateID: first.UpdateID, message: &combined, raws: raws}, last
}

// rawsOf returns the message exactly as Telegram sent it, or nothing when the
// update carried no message.
func rawsOf(message *tg.Message) []json.RawMessage {
	if message == nil || len(message.Raw) == 0 {
		return nil
	}
	return []json.RawMessage{message.Raw}
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

func batchableMessage(message *tg.Message) bool {
	if message == nil || message.From == nil || message.From.IsBot || message.Chat.Type != "private" || message.Text == "" {
		return false
	}
	return !strings.HasPrefix(strings.TrimSpace(message.Text), "/")
}

func samePaste(previous, candidate *tg.Message) bool {
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
