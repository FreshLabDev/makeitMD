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

	"github.com/FreshLabDev/makeitMD/internal/build"
	"github.com/FreshLabDev/makeitMD/internal/db"
	"github.com/FreshLabDev/makeitMD/internal/metrics"
	"github.com/FreshLabDev/makeitMD/internal/richmarkdown"
)

const (
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
	SetMyCommandsForScope(context.Context, []tg.BotCommand, *tg.BotCommandScope) error
	GetUpdates(context.Context, int64, int) ([]tg.Update, error)
	SendMessage(context.Context, int64, string, *tg.InlineKeyboardMarkup) (tg.Message, error)
	SendPlainText(context.Context, int64, string) (tg.Message, error)
	SendEphemeralMessage(context.Context, int64, int64, int64, string, *tg.InlineKeyboardMarkup) (tg.Message, error)
	SendRichMarkdown(context.Context, int64, string, *tg.InlineKeyboardMarkup, ...tg.RichOption) (tg.Message, error)
	EditMessageText(context.Context, int64, int64, string, *tg.InlineKeyboardMarkup) error
	AnswerCallbackQuery(context.Context, string, string) error
}

// pollTimeout is the long-poll duration in seconds. The client derives its own
// HTTP deadline from it.
const pollTimeout = 50

// privateCommands is the menu in a direct message, which is where the whole bot
// lives.
var privateCommands = []tg.BotCommand{{Command: "start", Description: "Open the makeitMD panel"}}

// groupCommands keep one command in groups rather than none. An empty list
// would be tidier on paper, but somebody who adds makeitMD to a group would
// then find no menu, no reply and nothing saying why. `IsEphemeral` is what
// makes a Bot API 10.3 client send the command itself as an ephemeral message,
// which is both what authorizes the reply and what keeps the whole exchange
// invisible to everyone except the person who typed it -- so the redirect costs
// the group no message at all.
var groupCommands = []tg.BotCommand{{Command: "start", Description: "Open makeitMD privately", IsEphemeral: true}}

// noCommands clears the scope it is published to. It must not be nil: the API
// takes an empty JSON array, and a nil slice marshals to null.
var noCommands = []tg.BotCommand{}

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
	build        build.Info
	username     string
	lastPollUnix atomic.Int64
}

// New takes the build the binary was stamped with, so the About screen reports
// the same version `/healthz` does, and the bot's own username, which is the
// deep link a group /start hands back.
func New(client Telegram, data Store, log *slog.Logger, info build.Info, username string) *Bot {
	return &Bot{telegram: client, store: data, log: log, build: info, username: username}
}

// registerCommands publishes one list per scope instead of a single global one.
// The bot renders Markdown in private chats only, and a global list offers the
// command everywhere -- including chat types where tapping it does nothing.
// The default scope is cleared rather than left alone: makeitMD used to publish
// there, and the two explicit scopes below already cover every chat a bot can
// be in, so anything still registered on the default is a description nothing
// reaches. A failure here is logged and not fatal: a bot that renders Markdown
// without a menu still works.
func (b *Bot) registerCommands(ctx context.Context) {
	scopes := []struct {
		scope    string
		commands []tg.BotCommand
	}{
		{"all_private_chats", privateCommands},
		{"all_group_chats", groupCommands},
		{"default", noCommands},
	}
	for _, published := range scopes {
		if err := b.telegram.SetMyCommandsForScope(ctx, published.commands,
			&tg.BotCommandScope{Type: published.scope}); err != nil {
			b.log.Warn("set telegram commands failed", "scope", published.scope, "error", err)
		}
	}
}

func (b *Bot) Run(ctx context.Context) error {
	b.registerCommands(ctx)
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
			// A callback query carries no message of its own to group with the
			// next update, so panel taps are routed on their own.
			updateID := updates[index].UpdateID
			lastIndex := index
			var handleErr error
			if callback := updates[index].Callback; callback != nil {
				handleErr = b.handleCallback(ctx, callback)
			} else {
				var grouped paste
				grouped, lastIndex = groupTextUpdates(updates, index)
				handleErr = b.handle(ctx, grouped)
			}
			if handleErr != nil {
				if ctx.Err() != nil {
					return nil
				}
				if updateID == lastFailedUpdate {
					failedRetries++
				} else {
					lastFailedUpdate = updateID
					failedRetries = 1
				}
				if failedRetries <= maxUpdateRetries {
					b.log.Error("telegram update failed; will retry", "update_id", updateID, "attempt", failedRetries, "error", handleErr)
					if !wait(ctx, jitterDuration(250*time.Millisecond)) {
						return nil
					}
					break
				}
				b.log.Error("telegram update permanently failed; dropping", "update_id", updateID, "attempts", failedRetries, "error", handleErr)
				lastFailedUpdate = 0
				failedRetries = 0
			} else if updateID == lastFailedUpdate {
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
	if message == nil || message.From == nil || message.From.IsBot || message.Text == "" {
		return nil
	}
	if message.Chat.Type != "private" {
		return b.handleGroupStart(ctx, message)
	}
	if err := b.store.Touch(ctx, *message.From); err != nil {
		return err
	}
	if isStartCommand(message.Text) {
		return b.sendScreen(ctx, message.Chat.ID, rootScreen())
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

// handleGroupStart answers /start where makeitMD cannot work. The reply is
// ephemeral, so the group sees nothing; without an ephemeral id there is
// nothing to reply to -- an older client sent the command as an ordinary
// public message -- and staying silent beats posting a redirect everybody in
// the group has to read. Nothing is written to the database on this path: no
// conversion happened, and a person who never opened a private chat is not
// somebody this bot has anything to remember about.
func (b *Bot) handleGroupStart(ctx context.Context, message *tg.Message) error {
	if message.EphemeralMessageID == 0 || !isStartCommand(message.Text) {
		return nil
	}
	view := groupScreen(b.username)
	_, err := b.telegram.SendEphemeralMessage(ctx, message.Chat.ID, message.From.ID,
		message.EphemeralMessageID, view.text, view.markup)
	return err
}

// handleCallback moves the panel between its screens by editing the message the
// button is attached to, so the chat keeps one panel instead of a stack of them.
func (b *Bot) handleCallback(ctx context.Context, query *tg.CallbackQuery) error {
	// The spinner on a tapped button turns until the query is answered, so it
	// is answered before the edit rather than after it. A failure there is not
	// worth abandoning the navigation the person asked for.
	if err := b.telegram.AnswerCallbackQuery(ctx, query.ID, ""); err != nil {
		b.log.Warn("answer callback query failed", "error", err)
	}
	// The client models an absent callback message as a zero value rather than
	// nil, which would address chat 0. makeitMD only ever attaches buttons to a
	// private-chat message of its own, so anything else is not this panel.
	if query.Message.Chat.Type != "private" || query.Message.MessageID == 0 {
		return nil
	}
	view := b.screenFor(query.Data)
	return b.telegram.EditMessageText(ctx, query.Message.Chat.ID, query.Message.MessageID, view.text, view.markup)
}

// sendScreen posts a panel screen. Unlike the strings below it, panel text is
// HTML: the About card is a fixed, reviewed string that has to render a quote
// and a link, and it carries no user input that could break its own markup.
func (b *Bot) sendScreen(ctx context.Context, chatID int64, view screen) error {
	_, err := b.telegram.SendMessage(ctx, chatID, view.text, view.markup)
	return err
}

// isStartCommand accepts the bare command and the /start@bot form a group
// client sends.
func isStartCommand(text string) bool {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return false
	}
	return fields[0] == "/start" || strings.HasPrefix(fields[0], "/start@")
}

// sendText delivers one of this bot's own fixed strings with no parse mode.
// One of them is the message a user gets when their Markdown failed to render;
// sending it as HTML would make a stray angle bracket in that sentence fail
// the delivery too, and the person would get nothing at all.
func (b *Bot) sendText(ctx context.Context, chatID int64, text string) error {
	_, err := b.telegram.SendPlainText(ctx, chatID, text)
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
