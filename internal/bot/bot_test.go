// SPDX-License-Identifier: Apache-2.0
package bot

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/FreshLabDev/makeitMD/internal/db"
	"github.com/FreshLabDev/makeitMD/internal/telegram"
)

type fakeStore struct {
	status                         db.ConversionStatus
	touched, created, sent, failed int
}

func (s *fakeStore) Offset(context.Context) (int64, error)      { return 0, nil }
func (s *fakeStore) AdvanceOffset(context.Context, int64) error { return nil }
func (s *fakeStore) Touch(context.Context, telegram.User) error { s.touched++; return nil }
func (s *fakeStore) CreateConversion(context.Context, int64, telegram.Message, string) (int64, db.ConversionStatus, error) {
	s.created++
	status := s.status
	if status == "" {
		status = db.ConversionReceived
	}
	return 9, status, nil
}
func (s *fakeStore) MarkSent(context.Context, int64, string, telegram.Result, []telegram.DeliveryAttempt) error {
	s.sent++
	return nil
}
func (s *fakeStore) MarkFailed(context.Context, int64, string, string, telegram.Result, []telegram.DeliveryAttempt) error {
	s.failed++
	return nil
}

type fakeTelegram struct {
	textErr, richErr error
	texts            []string
	rich             []string
}

func (f *fakeTelegram) SetStartCommand(context.Context) error                        { return nil }
func (f *fakeTelegram) GetUpdates(context.Context, int64) ([]telegram.Update, error) { return nil, nil }
func (f *fakeTelegram) SendText(_ context.Context, _ int64, text string) error {
	f.texts = append(f.texts, text)
	return f.textErr
}
func (f *fakeTelegram) SendRichMarkdown(_ context.Context, _ int64, text string) (telegram.Result, error) {
	f.rich = append(f.rich, text)
	return telegram.Result(`{"message_id":99}`), f.richErr
}

func testUpdate(text string) telegram.Update {
	return telegram.Update{UpdateID: 5, Message: &telegram.Message{
		MessageID: 3, Chat: telegram.Chat{ID: 7, Type: "private"},
		From: &telegram.User{ID: 7, FirstName: "A"}, Text: text,
	}}
}

func newTestBot(client *fakeTelegram, store *fakeStore) *Bot {
	return New(client, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestStartSendFailureIsReturned(t *testing.T) {
	want := errors.New("temporary")
	client := &fakeTelegram{textErr: want}
	store := &fakeStore{}
	err := newTestBot(client, store).handle(context.Background(), testUpdate("/start"))
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
	if err := newTestBot(client, store).handle(context.Background(), testUpdate(source)); err != nil {
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
	update.Message.Entities = []telegram.MessageEntity{{Type: "bold", Offset: 0, Length: 6}}
	if err := newTestBot(client, store).handle(context.Background(), update); err != nil {
		t.Fatal(err)
	}
	if got, want := client.rich[0], "<b>styled</b> text"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestAlreadySentConversionIsNotDuplicated(t *testing.T) {
	client := &fakeTelegram{}
	store := &fakeStore{status: db.ConversionSent}
	if err := newTestBot(client, store).handle(context.Background(), testUpdate("hello")); err != nil {
		t.Fatal(err)
	}
	if len(client.rich) != 0 || store.sent != 0 {
		t.Fatalf("duplicate send: client=%+v store=%+v", client, store)
	}
}

func TestBadMarkdownIsMarkedFailedAndExplained(t *testing.T) {
	client := &fakeTelegram{richErr: &telegram.APIError{StatusCode: 400, ErrorCode: 400, Description: "bad markdown"}}
	store := &fakeStore{}
	if err := newTestBot(client, store).handle(context.Background(), testUpdate("**broken")); err != nil {
		t.Fatal(err)
	}
	if store.failed != 1 || len(client.texts) != 1 || client.texts[0] != errorText {
		t.Fatalf("client=%+v store=%+v", client, store)
	}
}

func TestGroupTextUpdatesJoinsTelegramPasteChunks(t *testing.T) {
	updates := []telegram.Update{
		testUpdateWithIDs(10, 20, 100, "# Project"),
		testUpdateWithIDs(11, 21, 100, "first part"),
		testUpdateWithIDs(12, 22, 101, "second part"),
	}
	grouped, last := groupTextUpdates(updates, 0)
	if last != 2 {
		t.Fatalf("last=%d", last)
	}
	if got, want := grouped.Message.Text, "# Project\nfirst part\nsecond part"; got != want {
		t.Fatalf("text=%q want=%q", got, want)
	}
}

func TestGroupTextUpdatesRebasesEntityOffsets(t *testing.T) {
	updates := []telegram.Update{
		testUpdateWithIDs(10, 20, 100, "😀 one"),
		testUpdateWithIDs(11, 21, 100, "two"),
	}
	updates[1].Message.Entities = []telegram.MessageEntity{{Type: "bold", Offset: 0, Length: 3}}
	grouped, _ := groupTextUpdates(updates, 0)
	if got, want := grouped.Message.Entities[0].Offset, 7; got != want {
		t.Fatalf("offset=%d want=%d", got, want)
	}
}

func TestGroupTextUpdatesDoesNotJoinSeparateMessages(t *testing.T) {
	updates := []telegram.Update{
		testUpdateWithIDs(10, 20, 100, "first"),
		testUpdateWithIDs(11, 22, 100, "not consecutive"),
	}
	_, last := groupTextUpdates(updates, 0)
	if last != 0 {
		t.Fatalf("last=%d, want 0", last)
	}
}

func testUpdateWithIDs(updateID, messageID, date int64, text string) telegram.Update {
	update := testUpdate(text)
	update.UpdateID = updateID
	update.Message.MessageID = messageID
	update.Message.Date = date
	return update
}
