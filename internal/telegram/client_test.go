// SPDX-License-Identifier: Apache-2.0
package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSendRichMarkdownPreservesInput(t *testing.T) {
	t.Parallel()
	const input = "# Heading\n\n- **exact** `text`\n\n| a | b |\n|---|---|\n| 1 | 2 |"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bottest-token/sendRichMessage" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		var body struct {
			ChatID      int64 `json:"chat_id"`
			RichMessage struct {
				Markdown string `json:"markdown"`
			} `json:"rich_message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.ChatID != 42 {
			t.Fatalf("chat_id = %d", body.ChatID)
		}
		if body.RichMessage.Markdown != input {
			t.Fatalf("markdown changed:\nwant %q\n got %q", input, body.RichMessage.Markdown)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiBase = server.URL
	result, err := client.SendRichMarkdown(context.Background(), 42, input)
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != `{"message_id":1}` {
		t.Fatalf("result=%s", result)
	}
}

func TestGetUpdatesDecodesTelegramEnvelope(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("offset") != "12" {
			t.Fatalf("offset = %q", r.URL.Query().Get("offset"))
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":[{"update_id":12,"message":{"message_id":3,"from":{"id":7,"is_bot":false,"first_name":"A"},"chat":{"id":7,"type":"private"},"text":"# hi"}}]}`))
	}))
	defer server.Close()
	client := NewClient("token")
	client.apiBase = server.URL
	updates, err := client.GetUpdates(context.Background(), 12)
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 1 || updates[0].UpdateID != 12 || updates[0].Message.Text != "# hi" {
		t.Fatalf("unexpected updates: %+v", updates)
	}
	if len(updates[0].Message.RawMessages) != 1 || !strings.Contains(string(updates[0].Message.RawMessages[0]), `"text":"# hi"`) {
		t.Fatalf("raw message not preserved: %s", updates[0].Message.RawMessages)
	}
}

func TestRateLimitRetriesOnceAndHonorsRetryAfter(t *testing.T) {
	t.Parallel()
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":2}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	}))
	defer server.Close()
	client := NewClient("token")
	client.apiBase = server.URL
	var slept time.Duration
	client.sleep = func(_ context.Context, duration time.Duration) error {
		slept = duration
		return nil
	}
	if err := client.SendText(context.Background(), 7, "hello"); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || slept != 2*time.Second {
		t.Fatalf("calls=%d slept=%s", calls, slept)
	}
}

func TestTransportErrorRedactsTokenAndKeepsCause(t *testing.T) {
	t.Parallel()
	const token = "123456:secret-token"
	client := NewClient(token)
	client.apiBase = "http://127.0.0.1:1"
	client.sleep = func(_ context.Context, _ time.Duration) error { return nil }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.GetUpdates(ctx, 0)
	if err == nil {
		t.Fatal("expected transport error")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("error exposed token: %q", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation cause lost: %v", err)
	}
}

func TestAPIErrorDoesNotExposeTokenOrURL(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: can't parse rich message"}`))
	}))
	defer server.Close()

	client := NewClient("secret-token")
	client.apiBase = server.URL
	_, err := client.SendRichMarkdown(context.Background(), 42, "**broken")
	if err == nil || err.Error() != "telegram sendRichMessage failed: Bad Request: can't parse rich message" {
		t.Fatalf("unexpected error: %v", err)
	}
}
