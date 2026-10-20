// SPDX-License-Identifier: Apache-2.0
package bot

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/FreshLabDev/tg"

	"github.com/FreshLabDev/makeitMD/internal/db"
	"github.com/FreshLabDev/makeitMD/internal/i18n"
)

// This is a download budget, not Telegram's rendered-text limit. Markup and
// URLs count towards these bytes; Telegram decides whether the result fits its
// rich-message limits. No partial file is ever rendered.
const (
	maxDocumentBytes    = 128 * 1024
	maxDocumentAttempts = 4 // Initial request plus up to three retries.
	documentTimeout     = 15 * time.Second
)

func (b *Bot) handleDocument(ctx context.Context, p paste) error {
	message := p.message
	status, err := b.store.ConversionStatus(ctx, p.updateID)
	if err != nil {
		return err
	}
	switch status {
	case db.ConversionSent:
		return nil
	case db.ConversionFailed:
		return b.sendText(ctx, message.Chat.ID, b.renderFailedText(ctx, *message.From))
	}
	source, key, err := b.documentSource(ctx, message.Document)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// An API response can echo private content. Keep filenames, paths,
		// captions and raw transport responses out of application logs.
		b.log.Warn("document download failed")
		key = "msg.file_failed"
	}
	if key != "" {
		return b.sendText(ctx, message.Chat.ID, i18n.T(b.resolveLang(ctx, *message.From), key))
	}
	// Keep the original document/caption in raws, and the decoded file contents
	// as source_text. The synthetic combined message cannot claim the original
	// document-only JSON as its own raw representation.
	combined := *message
	combined.Text = source
	combined.Entities = nil
	combined.Raw = nil
	p.message = &combined
	return b.convert(ctx, p, strings.TrimPrefix(source, "\uFEFF"), false)
}

// documentSource accepts text by content, not by a user-controlled extension
// or MIME type. It never executes, extracts, or fetches links from a file.
// key describes an expected input rejection; err describes a failed transfer.
func (b *Bot) documentSource(ctx context.Context, document *tg.Document) (source, key string, err error) {
	if document.FileSize > maxDocumentBytes {
		return "", "msg.file_too_large", nil
	}
	if document.FileID == "" {
		return "", "msg.file_failed", nil
	}
	ctx, cancel := context.WithTimeout(ctx, documentTimeout)
	defer cancel()
	file, err := b.telegram.GetFile(ctx, document.FileID)
	if err != nil {
		return "", "", err
	}
	if file.FileSize > maxDocumentBytes {
		return "", "msg.file_too_large", nil
	}
	// A private random directory avoids trusting Telegram's filename and
	// contains even incomplete downloads until the deferred cleanup runs.
	dir, err := os.MkdirTemp("", "makeitmd-document-*")
	if err != nil {
		return "", "", fmt.Errorf("create document directory: %w", err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "source")
	if err := b.downloadDocument(ctx, file.FilePath, path); err != nil {
		if errors.Is(err, tg.ErrFileTooLarge) {
			return "", "msg.file_too_large", nil
		}
		return "", "", err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("read downloaded document: %w", err)
	}
	if len(content) > maxDocumentBytes {
		return "", "msg.file_too_large", nil
	}
	if !utf8.Valid(content) {
		return "", "msg.file_not_text", nil
	}
	source = string(content)
	// MIME sniffing can mistake ordinary words such as "ID3" and "OTTO" for
	// media headers. Validate the text itself instead. PDF is a container that
	// can consist entirely of printable ASCII; reject its explicit signature.
	// Binary ZIP/Office and media data are caught by UTF-8/control validation.
	if strings.HasPrefix(strings.TrimPrefix(source, "\uFEFF"), "%PDF-") {
		return "", "msg.file_not_text", nil
	}
	for _, r := range source {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return "", "msg.file_not_text", nil
		}
	}
	if strings.TrimSpace(strings.TrimPrefix(source, "\uFEFF")) == "" {
		return "", "msg.file_empty", nil
	}
	return source, "", nil
}

// Downloading is idempotent, unlike sending a reply. The shared client retries
// getFile but not the file stream, so retry transient stream failures here,
// within the same overall deadline. Never let raw download errors reach logs.
func (b *Bot) downloadDocument(ctx context.Context, source, destination string) error {
	for attempt := 1; ; attempt++ {
		err := b.telegram.DownloadToFile(ctx, source, destination, maxDocumentBytes)
		if err == nil || ctx.Err() != nil || attempt == maxDocumentAttempts || errors.Is(err, tg.ErrFileTooLarge) {
			return err
		}
		var apiErr *tg.APIError
		if errors.As(err, &apiErr) {
			code := apiErr.ErrorCode
			if code == 0 {
				code = apiErr.StatusCode
			}
			if code != http.StatusTooManyRequests && code < 500 {
				return err
			}
		}
		delay := tg.RetryAfter(err)
		if delay <= 0 {
			delay = jitterDuration(time.Duration(attempt) * 200 * time.Millisecond)
		}
		if !wait(ctx, delay) {
			return ctx.Err()
		}
	}
}
