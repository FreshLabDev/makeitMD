// SPDX-License-Identifier: Apache-2.0
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	token   string
	apiBase string
	http    *http.Client
	timeout time.Duration
	sleep   func(context.Context, time.Duration) error
}

func NewClient(token string) *Client {
	return &Client{
		token:   token,
		apiBase: "https://api.telegram.org",
		http:    &http.Client{},
		timeout: 30 * time.Second,
		sleep:   sleep,
	}
}

func (c *Client) GetUpdates(ctx context.Context, offset int64) ([]Update, error) {
	values := url.Values{
		"timeout":         {"50"},
		"allowed_updates": {`["message"]`},
	}
	if offset > 0 {
		values.Set("offset", strconv.FormatInt(offset, 10))
	}
	var updates []Update
	if err := c.get(ctx, "getUpdates", values, &updates, 65*time.Second); err != nil {
		return nil, err
	}
	return updates, nil
}

func (c *Client) SetStartCommand(ctx context.Context) error {
	payload := map[string]any{"commands": []map[string]string{{
		"command": "start", "description": "Start the bot",
	}}}
	return c.post(ctx, "setMyCommands", payload, nil)
}

func (c *Client) SendText(ctx context.Context, chatID int64, text string) error {
	return c.post(ctx, "sendMessage", map[string]any{
		"chat_id": chatID,
		"text":    text,
	}, nil)
}

func (c *Client) SendRichMarkdown(ctx context.Context, chatID int64, markdown string) error {
	return c.post(ctx, "sendRichMessage", map[string]any{
		"chat_id": chatID,
		"rich_message": map[string]any{
			"markdown": markdown,
		},
	}, nil)
}

func (c *Client) get(ctx context.Context, method string, values url.Values, out any, timeout time.Duration) error {
	const maxAttempts = 4
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		retryAfter, err := c.attempt(ctx, timeout, func(attemptCtx context.Context) (*http.Request, error) {
			return http.NewRequestWithContext(attemptCtx, http.MethodGet, c.endpoint(method)+"?"+values.Encode(), nil)
		}, method, out)
		if err == nil {
			return nil
		}
		lastErr = err
		if ctx.Err() != nil || attempt == maxAttempts || !retryableError(err) {
			return err
		}
		delay := retryAfter
		if delay <= 0 {
			delay = retryDelay(attempt)
		}
		if err := c.sleep(ctx, delay); err != nil {
			return lastErr
		}
	}
	return lastErr
}

// POST retries only when Telegram explicitly confirms that the request was
// rate-limited. Retrying a generic transport failure could duplicate a message
// whose successful response was lost in transit.
func (c *Client) post(ctx context.Context, method string, payload any, out any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		retryAfter, err := c.attempt(ctx, c.timeout, func(attemptCtx context.Context) (*http.Request, error) {
			req, err := http.NewRequestWithContext(attemptCtx, http.MethodPost, c.endpoint(method), bytes.NewReader(raw))
			if err != nil {
				return nil, err
			}
			req.Header.Set("Content-Type", "application/json")
			return req, nil
		}, method, out)
		if err == nil {
			return nil
		}
		lastErr = err
		if retryAfter <= 0 || attempt == 1 {
			return err
		}
		if err := c.sleep(ctx, retryAfter); err != nil {
			return lastErr
		}
	}
	return lastErr
}

func (c *Client) attempt(ctx context.Context, timeout time.Duration, build func(context.Context) (*http.Request, error), method string, out any) (time.Duration, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := build(attemptCtx)
	if err != nil {
		return 0, c.redactError(err)
	}
	return c.do(method, req, out)
}

func (c *Client) do(method string, req *http.Request, out any) (time.Duration, error) {
	response, err := c.http.Do(req)
	if err != nil {
		return 0, c.redactError(err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		apiErr := parseAPIError(method, response.StatusCode, response.Body)
		return apiErr.RetryAfter, apiErr
	}
	var envelope struct {
		OK          bool   `json:"ok"`
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
		Parameters  struct {
			RetryAfter int `json:"retry_after"`
		} `json:"parameters"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&envelope); err != nil {
		return 0, fmt.Errorf("decode telegram %s response: %w", method, err)
	}
	if !envelope.OK {
		apiErr := &APIError{Method: method, StatusCode: response.StatusCode, ErrorCode: envelope.ErrorCode, Description: envelope.Description}
		apiErr.RetryAfter = time.Duration(envelope.Parameters.RetryAfter) * time.Second
		return apiErr.RetryAfter, apiErr
	}
	if out != nil && len(envelope.Result) > 0 {
		if err := json.Unmarshal(envelope.Result, out); err != nil {
			return 0, fmt.Errorf("decode telegram %s result: %w", method, err)
		}
	}
	return 0, nil
}

func (c *Client) endpoint(method string) string {
	return strings.TrimRight(c.apiBase, "/") + "/bot" + c.token + "/" + method
}

func (c *Client) redactError(err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	if c.token != "" {
		message = strings.ReplaceAll(message, c.token, "***")
	}
	cause := err
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		cause = urlErr.Err
	}
	return &transportError{message: message, cause: cause}
}

type transportError struct {
	message string
	cause   error
}

func (e *transportError) Error() string { return e.message }
func (e *transportError) Unwrap() error { return e.cause }

func parseAPIError(method string, statusCode int, body io.Reader) *APIError {
	raw, _ := io.ReadAll(io.LimitReader(body, 4096))
	var payload struct {
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
		Parameters  struct {
			RetryAfter int `json:"retry_after"`
		} `json:"parameters"`
	}
	description := strings.TrimSpace(string(raw))
	if err := json.Unmarshal(raw, &payload); err == nil && payload.Description != "" {
		description = payload.Description
	}
	return &APIError{
		Method: method, StatusCode: statusCode, ErrorCode: payload.ErrorCode,
		Description: description, RetryAfter: time.Duration(payload.Parameters.RetryAfter) * time.Second,
	}
}

func retryableError(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusTooManyRequests || apiErr.StatusCode >= 500
	}
	return !errors.Is(err, context.Canceled)
}

func retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 3 {
		attempt = 3
	}
	return jitterDuration(500 * time.Millisecond << (attempt - 1))
}

func jitterDuration(duration time.Duration) time.Duration {
	if duration <= 0 {
		return duration
	}
	half := int64(duration / 2)
	return duration - time.Duration(half) + time.Duration(rand.Int63n(2*half+1))
}

func sleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
