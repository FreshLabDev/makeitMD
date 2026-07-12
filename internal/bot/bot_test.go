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
func (s *fakeStore) CreateConversion(context.Context, int64, telegram.Message) (int64, db.ConversionStatus, error) {
	s.created++
	status := s.status
	if status == "" {
		status = db.ConversionReceived
	}
	return 9, status, nil
}
func (s *fakeStore) MarkSent(context.Context, int64) error           { s.sent++; return nil }
func (s *fakeStore) MarkFailed(context.Context, int64, string) error { s.failed++; return nil }

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
func (f *fakeTelegram) SendRichMarkdown(_ context.Context, _ int64, text string) error {
	f.rich = append(f.rich, text)
	return f.richErr
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
