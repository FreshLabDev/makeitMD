// SPDX-License-Identifier: Apache-2.0
package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/FreshLabDev/tg"

	"github.com/FreshLabDev/makeitMD/internal/build"
)

// Exercise the pinned shared client's real getFile/download/sendRichMessage
// protocol without reaching Telegram or using credentials.
func TestDocumentThroughSharedHTTPClient(t *testing.T) {
	for _, oversize := range []bool{false, true} {
		t.Run(fmt.Sprintf("oversize=%v", oversize), func(t *testing.T) {
			source := "/start\n\n$\\frac{1}{2}$\n\n```math\nE=mc^2\n```\n"
			if oversize {
				source = strings.Repeat("a", maxDocumentBytes+1)
			}
			var downloads atomic.Int32
			rendered := make(chan string, 1)
			plain := make(chan string, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/botTEST/getFile":
					if r.URL.Query().Get("file_id") != "file" {
						t.Error("file_id missing")
					}
					fmt.Fprint(w, `{"ok":true,"result":{"file_id":"file","file_path":"documents/source.md"}}`)
				case "/file/botTEST/documents/source.md":
					downloads.Add(1)
					fmt.Fprint(w, source)
				case "/botTEST/sendRichMessage":
					var body struct {
						RichMessage struct {
							Markdown            string `json:"markdown"`
							SkipEntityDetection bool   `json:"skip_entity_detection"`
						} `json:"rich_message"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if body.RichMessage.SkipEntityDetection {
						t.Error("entity detection was disabled")
					}
					rendered <- body.RichMessage.Markdown
					fmt.Fprint(w, `{"ok":true,"result":{"message_id":99,"chat":{"id":7,"type":"private"}}}`)
				case "/botTEST/sendMessage":
					var body struct {
						Text string `json:"text"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					plain <- body.Text
					fmt.Fprint(w, `{"ok":true,"result":{"message_id":100}}`)
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			client := tg.New("TEST", tg.WithAPIBase(server.URL))
			store := &fakeStore{}
			bot := New(client, store, slog.New(slog.DiscardHandler), build.Info{}, "test_bot")
			if err := bot.handle(context.Background(), testPaste(documentUpdate(t))); err != nil {
				t.Fatal(err)
			}
			if downloads.Load() != 1 {
				t.Fatalf("downloads=%d", downloads.Load())
			}
			if oversize {
				if len(rendered) != 0 || len(plain) != 1 || store.created != 0 {
					t.Fatal("shared streaming limit did not reject oversized content")
				}
			} else {
				if len(rendered) != 1 || <-rendered != source || len(plain) != 0 || store.sent != 1 {
					t.Fatal("document did not reach rich API unchanged")
				}
			}
		})
	}
}
