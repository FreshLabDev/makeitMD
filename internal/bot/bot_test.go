// SPDX-License-Identifier: Apache-2.0
package bot

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/FreshLabDev/makeitMD/internal/build"
	"github.com/FreshLabDev/makeitMD/internal/db"
	"github.com/FreshLabDev/tg"
)

type fakeStore struct {
	status                         db.ConversionStatus
	touched, created, sent, failed int
}

func (s *fakeStore) Offset(context.Context) (int64, error)      { return 0, nil }
func (s *fakeStore) AdvanceOffset(context.Context, int64) error { return nil }
func (s *fakeStore) Touch(context.Context, tg.User) error       { s.touched++; return nil }
func (s *fakeStore) CreateConversion(context.Context, int64, tg.Message, []json.RawMessage, string) (int64, db.ConversionStatus, error) {
	s.created++
	status := s.status
	if status == "" {
		status = db.ConversionReceived
	}
	return 9, status, nil
}
func (s *fakeStore) MarkSent(context.Context, int64, string, db.Result, []db.DeliveryAttempt) error {
	s.sent++
	return nil
}
func (s *fakeStore) MarkFailed(context.Context, int64, string, string, db.Result, []db.DeliveryAttempt) error {
	s.failed++
	return nil
}

type sentMessage struct {
	text   string
	markup *tg.InlineKeyboardMarkup
}

type fakeTelegram struct {
	textErr, richErr, messageErr error
	texts                        []string
	rich                         []string
	messages                     []sentMessage
	ephemeral                    []sentMessage
	edits                        []sentMessage
	answered                     []string
	scopes                       map[string][]tg.BotCommand
}

func (f *fakeTelegram) SetMyCommandsForScope(_ context.Context, commands []tg.BotCommand, scope *tg.BotCommandScope) error {
	if f.scopes == nil {
		f.scopes = map[string][]tg.BotCommand{}
	}
	name := "none"
	if scope != nil {
		name = scope.Type
	}
	f.scopes[name] = commands
	return nil
}
func (f *fakeTelegram) GetUpdates(context.Context, int64, int) ([]tg.Update, error) { return nil, nil }
func (f *fakeTelegram) SendMessage(_ context.Context, _ int64, text string, markup *tg.InlineKeyboardMarkup) (tg.Message, error) {
	f.messages = append(f.messages, sentMessage{text: text, markup: markup})
	return tg.Message{MessageID: 42}, f.messageErr
}
func (f *fakeTelegram) SendPlainText(_ context.Context, _ int64, text string) (tg.Message, error) {
	f.texts = append(f.texts, text)
	return tg.Message{}, f.textErr
}
func (f *fakeTelegram) SendEphemeralMessage(_ context.Context, _, _, _ int64, text string, markup *tg.InlineKeyboardMarkup) (tg.Message, error) {
	f.ephemeral = append(f.ephemeral, sentMessage{text: text, markup: markup})
	return tg.Message{EphemeralMessageID: 8}, nil
}
func (f *fakeTelegram) SendRichMarkdown(_ context.Context, _ int64, text string, _ *tg.InlineKeyboardMarkup, _ ...tg.RichOption) (tg.Message, error) {
	f.rich = append(f.rich, text)
	return tg.Message{MessageID: 99, Raw: json.RawMessage(`{"message_id":99}`)}, f.richErr
}
func (f *fakeTelegram) EditMessageText(_ context.Context, _, _ int64, text string, markup *tg.InlineKeyboardMarkup) error {
	f.edits = append(f.edits, sentMessage{text: text, markup: markup})
	return nil
}
func (f *fakeTelegram) AnswerCallbackQuery(_ context.Context, id, _ string) error {
	f.answered = append(f.answered, id)
	return nil
}

// testPaste turns one update into what the run loop hands to handle.
func testPaste(update tg.Update) paste {
	grouped, _ := groupTextUpdates([]tg.Update{update}, 0)
	return grouped
}

func testUpdate(text string) tg.Update {
	return tg.Update{UpdateID: 5, Message: &tg.Message{
		MessageID: 3, Chat: tg.Chat{ID: 7, Type: "private"},
		From: &tg.User{ID: 7, FirstName: "A"}, Text: text,
	}}
}

func newTestBot(client *fakeTelegram, store *fakeStore) *Bot {
	return New(client, store, slog.New(slog.NewTextHandler(io.Discard, nil)),
		build.Info{Version: "v9.9.9", Commit: "abcdef1234"}, "makeitMD_bot")
}

func TestStartSendFailureIsReturned(t *testing.T) {
	want := errors.New("temporary")
	client := &fakeTelegram{messageErr: want}
	store := &fakeStore{}
	err := newTestBot(client, store).handle(context.Background(), testPaste(testUpdate("/start")))
	if !errors.Is(err, want) {
		t.Fatalf("error=%v", err)
	}
	if store.touched != 1 || store.created != 0 {
		t.Fatalf("store=%+v", store)
	}
}

func TestMarkdownIsStoredThenSentUnchanged(t *testing.T) {
	client := &fakeTelegram{}
	store := &fakeStore{}
	const source = "# exact\n\n- **Markdown**"
	if err := newTestBot(client, store).handle(context.Background(), testPaste(testUpdate(source))); err != nil {
		t.Fatal(err)
	}
	if len(client.rich) != 1 || client.rich[0] != source {
		t.Fatalf("rich=%q", client.rich)
	}
	if store.created != 1 || store.sent != 1 {
		t.Fatalf("store=%+v", store)
	}
}

func TestConsumedTelegramEntitiesAreRestored(t *testing.T) {
	client := &fakeTelegram{}
	store := &fakeStore{}
	update := testUpdate("styled text")
	update.Message.Entities = []tg.MessageEntity{{Type: "bold", Offset: 0, Length: 6}}
	if err := newTestBot(client, store).handle(context.Background(), testPaste(update)); err != nil {
		t.Fatal(err)
	}
	if got, want := client.rich[0], "<b>styled</b> text"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestAlreadySentConversionIsNotDuplicated(t *testing.T) {
	client := &fakeTelegram{}
	store := &fakeStore{status: db.ConversionSent}
	if err := newTestBot(client, store).handle(context.Background(), testPaste(testUpdate("hello"))); err != nil {
		t.Fatal(err)
	}
	if len(client.rich) != 0 || store.sent != 0 {
		t.Fatalf("duplicate send: client=%+v store=%+v", client, store)
	}
}

func TestBadMarkdownIsMarkedFailedAndExplained(t *testing.T) {
	client := &fakeTelegram{richErr: &tg.APIError{StatusCode: 400, ErrorCode: 400, Description: "bad markdown"}}
	store := &fakeStore{}
	if err := newTestBot(client, store).handle(context.Background(), testPaste(testUpdate("**broken"))); err != nil {
		t.Fatal(err)
	}
	if store.failed != 1 || len(client.texts) != 1 || client.texts[0] != errorText {
		t.Fatalf("client=%+v store=%+v", client, store)
	}
}

func TestGroupTextUpdatesJoinsTelegramPasteChunks(t *testing.T) {
	updates := []tg.Update{
		testUpdateWithIDs(10, 20, 100, "# Project"),
		testUpdateWithIDs(11, 21, 100, "first part"),
		testUpdateWithIDs(12, 22, 101, "second part"),
	}
	grouped, last := groupTextUpdates(updates, 0)
	if last != 2 {
		t.Fatalf("last=%d", last)
	}
	if got, want := grouped.message.Text, "# Project\nfirst part\nsecond part"; got != want {
		t.Fatalf("text=%q want=%q", got, want)
	}
}

func TestGroupTextUpdatesRebasesEntityOffsets(t *testing.T) {
	updates := []tg.Update{
		testUpdateWithIDs(10, 20, 100, "😀 one"),
		testUpdateWithIDs(11, 21, 100, "two"),
	}
	updates[1].Message.Entities = []tg.MessageEntity{{Type: "bold", Offset: 0, Length: 3}}
	grouped, _ := groupTextUpdates(updates, 0)
	if got, want := grouped.message.Entities[0].Offset, 7; got != want {
		t.Fatalf("offset=%d want=%d", got, want)
	}
}

func TestGroupTextUpdatesDoesNotJoinSeparateMessages(t *testing.T) {
	updates := []tg.Update{
		testUpdateWithIDs(10, 20, 100, "first"),
		testUpdateWithIDs(11, 22, 100, "not consecutive"),
	}
	_, last := groupTextUpdates(updates, 0)
	if last != 0 {
		t.Fatalf("last=%d, want 0", last)
	}
}

func testUpdateWithIDs(updateID, messageID, date int64, text string) tg.Update {
	update := testUpdate(text)
	update.UpdateID = updateID
	update.Message.MessageID = messageID
	update.Message.Date = date
	return update
}

// The conversion record keeps each Telegram message exactly as it arrived, and
// a paste split across several messages keeps all of them.
func TestGroupedPasteKeepsEveryOriginal(t *testing.T) {
	updates := []tg.Update{mustUpdate(t, `{"update_id":1,"message":{"message_id":1,"date":100,"chat":{"id":7,"type":"private"},"from":{"id":7,"is_bot":false,"first_name":"A"},"text":"first part"}}`),
		mustUpdate(t, `{"update_id":2,"message":{"message_id":2,"date":100,"chat":{"id":7,"type":"private"},"from":{"id":7,"is_bot":false,"first_name":"A"},"text":"second part"}}`)}

	grouped, last := groupTextUpdates(updates, 0)
	if last != 1 {
		t.Fatalf("last = %d, want both messages grouped", last)
	}
	if len(grouped.raws) != 2 {
		t.Fatalf("raws = %d, want one per original message", len(grouped.raws))
	}
	for i, raw := range grouped.raws {
		var decoded map[string]any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("raw %d is not valid JSON: %v", i, err)
		}
		if decoded["message_id"] != float64(i+1) {
			t.Fatalf("raw %d holds message_id %v", i, decoded["message_id"])
		}
	}
	// The stitched message is synthetic; its own Raw would describe only the
	// first part, so it must not claim to be an original.
	if len(grouped.message.Raw) != 0 {
		t.Fatal("a stitched message must not carry the first message's raw bytes")
	}
}

func mustUpdate(t *testing.T, raw string) tg.Update {
	t.Helper()
	var update tg.Update
	if err := json.Unmarshal([]byte(raw), &update); err != nil {
		t.Fatal(err)
	}
	return update
}
