// SPDX-License-Identifier: Apache-2.0
package telegram

import "time"

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
	MessageID int64  `json:"message_id"`
	From      *User  `json:"from"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
}

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
}

func (e *APIError) Error() string {
	message := "telegram " + e.Method + " failed: " + e.Description
	if e.RetryAfter > 0 {
		message += " (retry after " + e.RetryAfter.String() + ")"
	}
	return message
}
