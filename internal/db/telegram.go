// SPDX-License-Identifier: Apache-2.0
package db

import "encoding/json"

// Result is a Telegram response kept exactly as it arrived. makeitMD stores
// one per conversion so a delivery can be explained later without guessing
// what the API actually said.
type Result = json.RawMessage

// DeliveryAttempt records one try at rendering a paste: the Markdown that was
// sent, and what came back. A conversion can hold several, because a 400 is
// retried with normalized Markdown.
type DeliveryAttempt struct {
	Markdown string          `json:"markdown"`
	Response json.RawMessage `json:"response,omitempty"`
	Error    string          `json:"error,omitempty"`
}
