// SPDX-License-Identifier: Apache-2.0
package telegram

import (
	"encoding/json"
	"time"
)

type User struct {
	ID           int64  `json:"id"`
	IsBot        bool   `json:"is_bot"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Username     string `json:"username"`
	LanguageCode string `json:"language_code"`
}

type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type Message struct {
	MessageID   int64             `json:"message_id"`
	Date        int64             `json:"date"`
	From        *User             `json:"from"`
	Chat        Chat              `json:"chat"`
	Text        string            `json:"text"`
	Entities    []MessageEntity   `json:"entities,omitempty"`
	RawMessages []json.RawMessage `json:"-"`
}

func (m *Message) UnmarshalJSON(data []byte) error {
	type message Message
	var decoded message
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*m = Message(decoded)
	m.RawMessages = []json.RawMessage{append(json.RawMessage(nil), data...)}
	return nil
}

type MessageEntity struct {
	Type          string `json:"type"`
	Offset        int    `json:"offset"`
	Length        int    `json:"length"`
	URL           string `json:"url,omitempty"`
	User          *User  `json:"user,omitempty"`
	Language      string `json:"language,omitempty"`
	CustomEmojiID string `json:"custom_emoji_id,omitempty"`
}

type Result = json.RawMessage

type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

type APIError struct {
	Method      string
	StatusCode  int
	ErrorCode   int
	Description string
	RetryAfter  time.Duration
	Response    json.RawMessage
}

func (e *APIError) Error() string {
	message := "telegram " + e.Method + " failed: " + e.Description
	if e.RetryAfter > 0 {
		message += " (retry after " + e.RetryAfter.String() + ")"
	}
	return message
}
