// SPDX-License-Identifier: Apache-2.0
package bot

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FreshLabDev/tg"

	"github.com/FreshLabDev/makeitMD/internal/db"
	"github.com/FreshLabDev/makeitMD/internal/i18n"
)

func documentUpdate(t *testing.T) tg.Update {
	t.Helper()
	return mustUpdate(t, `{"update_id":5,"message":{"message_id":3,"chat":{"id":7,"type":"private"},"from":{"id":7,"is_bot":false,"first_name":"A"},"document":{"file_id":"file","file_name":"../../source.md","mime_type":"application/octet-stream"},"caption":"/start caption","caption_entities":[{"type":"bold","offset":0,"length":6}]}}`)
}

func assertDownloadCleaned(t *testing.T, client *fakeTelegram) {
	t.Helper()
	if client.downloadPath == "" {
		t.Fatal("download did not run")
	}
	if _, err := os.Stat(filepath.Dir(client.downloadPath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary download directory remains: %v", err)
	}
}

func TestDocumentUsesTheSameRichReplyAndKeepsOriginalInput(t *testing.T) {
	const source = "\uFEFF# Документ 😀\r\n\r\n$x^2$\r\n\r\n$$E=mc^2$$\r\n"
	client := &fakeTelegram{fileContent: []byte(source)}
	store := &fakeStore{}
	update := documentUpdate(t)
	if err := newTestBot(client, store).handle(context.Background(), testPaste(update)); err != nil {
		t.Fatal(err)
	}
	if len(client.rich) != 1 || client.rich[0] != strings.TrimPrefix(source, "\uFEFF") || len(client.messages) != 0 {
		t.Fatalf("document source was changed or caption rendered: %+v", client)
	}
	if store.sourceMessage.Text != source || len(store.sourceMessage.Entities) != 0 || len(store.sourceMessage.Raw) != 0 {
		t.Fatalf("source was not kept exactly: %+v", store.sourceMessage)
	}
	if len(store.sourceRaws) != 1 || string(store.sourceRaws[0]) != string(update.Message.Raw) {
		t.Fatalf("original document metadata missing: %s", store.sourceRaws)
	}
	if store.sent != 1 || store.touched != 1 || client.downloadLimit != maxDocumentBytes {
		t.Fatalf("store=%+v downloadLimit=%d", store, client.downloadLimit)
	}
	deadline, ok := client.downloadContext.Deadline()
	if !ok || time.Until(deadline) > documentTimeout {
		t.Fatalf("download has no bounded deadline: %v %v", deadline, ok)
	}
	assertDownloadCleaned(t, client)
}

func TestTextDocumentDoesNotDependOnExtensionOrMIME(t *testing.T) {
	for _, name := range []string{"notes.md", "notes.txt", "README", "data.json", "table.csv", "formula.tex", "other.custom"} {
		t.Run(name, func(t *testing.T) {
			client := &fakeTelegram{fileContent: []byte("# text\n")}
			update := documentUpdate(t)
			update.Message.Document.FileName = name
			if err := newTestBot(client, &fakeStore{}).handle(context.Background(), testPaste(update)); err != nil {
				t.Fatal(err)
			}
			if len(client.rich) != 1 || client.rich[0] != "# text\n" {
				t.Fatalf("rich=%q", client.rich)
			}
			assertDownloadCleaned(t, client)
		})
	}
}

func TestDocumentInputRejectionsAreExplainedWithoutConversion(t *testing.T) {
	for _, tc := range []struct {
		name, key string
		content   []byte
	}{
		{"empty", "msg.file_empty", nil},
		{"blank", "msg.file_empty", []byte("\uFEFF \r\n\t")},
		{"invalid UTF-8", "msg.file_not_text", []byte{0xff, 0xfe, 'A', 0}},
		{"NUL", "msg.file_not_text", []byte("hello\x00world")},
		{"control", "msg.file_not_text", []byte("hello\x1bworld")},
		{"ASCII PDF", "msg.file_not_text", []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj")},
		{"oversized actual bytes", "msg.file_too_large", []byte(strings.Repeat("a", maxDocumentBytes+1))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeTelegram{fileContent: tc.content}
			store := &fakeStore{lang: "ru"}
			if err := newTestBot(client, store).handle(context.Background(), testPaste(documentUpdate(t))); err != nil {
				t.Fatal(err)
			}
			if len(client.texts) != 1 || client.texts[0] != i18n.T("ru", tc.key) || len(client.rich) != 0 || store.created != 0 {
				t.Fatalf("client=%+v store=%+v", client, store)
			}
			assertDownloadCleaned(t, client)
		})
	}
}

func TestDocumentSizeIsCheckedBeforeAndDuringDownload(t *testing.T) {
	for _, tc := range []struct {
		name               string
		declared, returned int64
		downloadErr        error
		files, downloads   int
	}{
		{"update metadata", maxDocumentBytes + 1, 0, nil, 0, 0},
		{"getFile metadata", 0, maxDocumentBytes + 1, nil, 1, 0},
		{"streaming limit", 0, 0, tg.ErrFileTooLarge, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			update := documentUpdate(t)
			update.Message.Document.FileSize = tc.declared
			client := &fakeTelegram{file: tg.File{FileSize: tc.returned}, downloadErr: tc.downloadErr}
			if err := newTestBot(client, &fakeStore{}).handle(context.Background(), testPaste(update)); err != nil {
				t.Fatal(err)
			}
			if client.files != tc.files || client.downloads != tc.downloads || len(client.texts) != 1 || client.texts[0] != i18n.T("en", "msg.file_too_large") {
				t.Fatalf("client=%+v", client)
			}
			if tc.downloads > 0 {
				assertDownloadCleaned(t, client)
			}
		})
	}
}

func TestDocumentDownloadFailureIsExplainedAndCleaned(t *testing.T) {
	client := &fakeTelegram{fileContent: []byte("partial"), downloadErr: errors.New("offline")}
	store := &fakeStore{}
	if err := newTestBot(client, store).handle(context.Background(), testPaste(documentUpdate(t))); err != nil {
		t.Fatal(err)
	}
	if len(client.texts) != 1 || client.texts[0] != i18n.T("en", "msg.file_failed") || store.created != 0 {
		t.Fatalf("client=%+v store=%+v", client, store)
	}
	assertDownloadCleaned(t, client)
}

func TestDocumentDownloadRetriesTransientFailures(t *testing.T) {
	for _, firstErr := range []error{
		&tg.APIError{StatusCode: 502, ErrorCode: 502},
		&tg.APIError{StatusCode: 429, ErrorCode: 429, RetryAfter: time.Millisecond},
	} {
		client := &fakeTelegram{fileContent: []byte("# recovered"), downloadErrors: []error{firstErr, nil}}
		store := &fakeStore{}
		if err := newTestBot(client, store).handle(context.Background(), testPaste(documentUpdate(t))); err != nil {
			t.Fatal(err)
		}
		if client.downloads != 2 || store.sent != 1 || len(client.texts) != 0 || len(client.rich) != 1 {
			t.Fatalf("temporary failure was acknowledged instead of retried: client=%+v store=%+v", client, store)
		}
		assertDownloadCleaned(t, client)
	}
}

func TestDocumentDownloadDoesNotRetryPermanentErrors(t *testing.T) {
	client := &fakeTelegram{downloadErr: &tg.APIError{StatusCode: 404, ErrorCode: 404}}
	if err := newTestBot(client, &fakeStore{}).handle(context.Background(), testPaste(documentUpdate(t))); err != nil {
		t.Fatal(err)
	}
	if client.downloads != 1 || len(client.texts) != 1 {
		t.Fatalf("client=%+v", client)
	}
	assertDownloadCleaned(t, client)
}

func TestDocumentAtByteLimitIsNotTruncatedOrRejected(t *testing.T) {
	source := strings.Repeat("😀", maxDocumentBytes/4)
	client := &fakeTelegram{fileContent: []byte(source)}
	if err := newTestBot(client, &fakeStore{}).handle(context.Background(), testPaste(documentUpdate(t))); err != nil {
		t.Fatal(err)
	}
	if len(client.rich) != 1 || client.rich[0] != source {
		t.Fatal("valid text at the byte limit did not reach the rich API unchanged")
	}
	assertDownloadCleaned(t, client)
}

func TestCancelledDocumentDoesNotSendAFailureNotification(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &fakeTelegram{fileErr: context.Canceled}
	err := newTestBot(client, &fakeStore{}).handle(ctx, testPaste(documentUpdate(t)))
	if !errors.Is(err, context.Canceled) || len(client.texts) != 0 || len(client.rich) != 0 {
		t.Fatalf("err=%v client=%+v", err, client)
	}
}

func TestTerminalDocumentReplayDoesNotDownloadAgain(t *testing.T) {
	for _, status := range []db.ConversionStatus{db.ConversionSent, db.ConversionFailed} {
		client := &fakeTelegram{fileErr: errors.New("file expired")}
		store := &fakeStore{status: status}
		if err := newTestBot(client, store).handle(context.Background(), testPaste(documentUpdate(t))); err != nil {
			t.Fatal(err)
		}
		if client.files != 0 || client.downloads != 0 || len(client.rich) != 0 || store.created != 0 {
			t.Fatalf("a terminal document was retried: client=%+v store=%+v", client, store)
		}
		if status == db.ConversionFailed && (len(client.texts) != 1 || client.texts[0] != i18n.T("en", "msg.render_failed")) {
			t.Fatalf("failed replay lost its explanation: %q", client.texts)
		}
	}
}

func TestDocumentLookupFailureStopsBeforeDownloading(t *testing.T) {
	want := errors.New("database unavailable")
	client := &fakeTelegram{}
	err := newTestBot(client, &fakeStore{lookupErr: want}).handle(context.Background(), testPaste(documentUpdate(t)))
	if !errors.Is(err, want) || client.files != 0 || len(client.rich) != 0 {
		t.Fatalf("err=%v client=%+v", err, client)
	}
}

func TestGroupDocumentHasNoDownloadsRepliesOrDatabaseWrites(t *testing.T) {
	update := documentUpdate(t)
	update.Message.Chat.Type = "supergroup"
	client := &fakeTelegram{}
	store := &fakeStore{}
	if err := newTestBot(client, store).handle(context.Background(), testPaste(update)); err != nil {
		t.Fatal(err)
	}
	if client.files != 0 || len(client.texts) != 0 || len(client.rich) != 0 || store.touched != 0 || store.created != 0 {
		t.Fatalf("group document caused work: client=%+v store=%+v", client, store)
	}
}

func TestMathSourcePassesThroughUnchanged(t *testing.T) {
	for _, source := range []string{`$x^2 + y^2$`, "$$\\frac{a}{b}$$", "```math\nE=mc^2\n```", `Price $5; code: \` + "`$x$`"} {
		client := &fakeTelegram{}
		if err := newTestBot(client, &fakeStore{}).handle(context.Background(), testPaste(testUpdate(source))); err != nil {
			t.Fatal(err)
		}
		if len(client.rich) != 1 || client.rich[0] != source {
			t.Fatalf("math source changed: %q", client.rich)
		}
	}
}
