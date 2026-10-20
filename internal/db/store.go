// SPDX-License-Identifier: Apache-2.0
package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/FreshLabDev/tg"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

type ConversionStatus string

type HealthStatus struct {
	Received int64 `json:"received"`
	Failed   int64 `json:"failed"`
}

const (
	ConversionReceived ConversionStatus = "received"
	ConversionSent     ConversionStatus = "sent"
	ConversionFailed   ConversionStatus = "failed"
)

func Connect(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

// botName is how makeitMD identifies itself to the shared core hub. Every
// statement that writes a claim on behalf of this bot -- presence and the
// language preference alike -- passes the same string, because core keys the
// claim by it and two spellings would be two bots.
const botName = "makeitmd"

func (s *Store) Touch(ctx context.Context, user tg.User) error {
	_, err := s.pool.Exec(ctx,
		`SELECT core.touch($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		botName, user.ID, nullString(user.Username), nullString(user.FirstName),
		nullString(user.LastName), nullString(user.LanguageCode), nil, nil, nil, nil, user.IsBot)
	return err
}

// SetLanguage records a manual language choice in the shared core hub, so the
// person who picked a language here is understood by the sibling bots too.
// The 'user'/'manual' literals stay inline on purpose: core takes
// core.pref_scope/core.lang_source enums in those positions, and pgx would
// send bound $n parameters as text, which PostgreSQL cannot match to the enum
// overloads.
func (s *Store) SetLanguage(ctx context.Context, userID int64, lang string) error {
	_, err := s.pool.Exec(ctx, `SELECT core.set_language($1,'user',$2,$3,'manual')`, botName, userID, lang)
	return err
}

// ClearLanguage deletes this bot's manual claim and lets the preference
// re-resolve, which in practice hands the decision back to the Telegram
// client's own language. It is what the "Follow Telegram" button does, and it
// clears only makeitMD's claim: a choice somebody made in a sibling bot is
// theirs to withdraw there.
func (s *Store) ClearLanguage(ctx context.Context, userID int64) error {
	_, err := s.pool.Exec(ctx, `SELECT core.clear_language($1,'user',$2)`, botName, userID)
	return err
}

// EffectiveLanguage reads the resolved language from the core hub. A missing
// preference is not an error: ok=false means "nobody has chosen, use the
// Telegram hint".
func (s *Store) EffectiveLanguage(ctx context.Context, userID int64) (string, bool, error) {
	var lang *string
	if err := s.pool.QueryRow(ctx, `SELECT core.effective_language($1,NULL,'user')`, userID).Scan(&lang); err != nil {
		return "", false, err
	}
	if lang == nil || *lang == "" {
		return "", false, nil
	}
	return *lang, true, nil
}

func (s *Store) CreateConversion(ctx context.Context, updateID int64, message tg.Message, raws []json.RawMessage, renderedMarkdown string) (int64, ConversionStatus, error) {
	var id int64
	var status ConversionStatus
	input, err := encodeTelegramInput(message, raws)
	if err != nil {
		return 0, "", fmt.Errorf("encode telegram input: %w", err)
	}
	err = s.pool.QueryRow(ctx, `
		INSERT INTO conversions (
			telegram_update_id, telegram_message_id, telegram_user_id,
			source_text, character_count, byte_count,
			telegram_input, rendered_markdown
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (telegram_update_id) DO UPDATE SET telegram_update_id = EXCLUDED.telegram_update_id
		RETURNING id, status
	`, updateID, message.MessageID, message.From.ID, message.Text,
		utf8.RuneCountInString(message.Text), len(message.Text), string(input), renderedMarkdown).Scan(&id, &status)
	return id, status, err
}

func (s *Store) MarkSent(ctx context.Context, id int64, renderedMarkdown string, response Result, attempts []DeliveryAttempt) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var userID int64
	var characters, bytes int64
	var sentAt time.Time
	err = tx.QueryRow(ctx, `
		UPDATE conversions
		SET status='sent', sent_at=now(), failed_at=NULL, error_code=NULL,
			rendered_markdown=$2, telegram_response=$3, telegram_attempts=$4
		WHERE id=$1 AND status='received'
		RETURNING telegram_user_id, character_count, byte_count, sent_at
	`, id, renderedMarkdown, nullableJSON(response), encodedJSON(attempts)).Scan(&userID, &characters, &bytes, &sentAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO user_stats (
			telegram_user_id, conversions, characters, bytes,
			first_conversion_at, last_conversion_at
		) VALUES ($1,1,$2,$3,$4,$4)
		ON CONFLICT (telegram_user_id) DO UPDATE SET
			conversions=user_stats.conversions+1,
			characters=user_stats.characters+EXCLUDED.characters,
			bytes=user_stats.bytes+EXCLUDED.bytes,
			first_conversion_at=LEAST(user_stats.first_conversion_at, EXCLUDED.first_conversion_at),
			last_conversion_at=GREATEST(user_stats.last_conversion_at, EXCLUDED.last_conversion_at)
	`, userID, characters, bytes, sentAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) MarkFailed(ctx context.Context, id int64, errorCode, renderedMarkdown string, response Result, attempts []DeliveryAttempt) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE conversions SET status='failed', failed_at=now(), error_code=$2,
			rendered_markdown=$3, telegram_response=$4, telegram_attempts=$5
		WHERE id=$1 AND status='received'
	`, id, errorCode, renderedMarkdown, nullableJSON(response), encodedJSON(attempts))
	return err
}

func encodedJSON(value any) any {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return string(encoded)
}

func nullableJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	if json.Valid(raw) {
		return string(raw)
	}
	encoded, _ := json.Marshal(map[string]string{"raw": string(raw)})
	return string(encoded)
}

// encodeTelegramInput stores what arrived. raws holds each original Telegram
// message; combined is the single message they were stitched into, which has
// no original of its own.
func encodeTelegramInput(message tg.Message, raws []json.RawMessage) ([]byte, error) {
	if len(raws) == 0 {
		return json.Marshal(message)
	}
	return json.Marshal(struct {
		Messages []json.RawMessage `json:"messages"`
		Combined tg.Message        `json:"combined"`
	}{Messages: raws, Combined: message})
}

func (s *Store) Offset(ctx context.Context) (int64, error) {
	var offset int64
	err := s.pool.QueryRow(ctx, `SELECT telegram_offset FROM runtime_state WHERE singleton=true`).Scan(&offset)
	return offset, err
}

func (s *Store) AdvanceOffset(ctx context.Context, offset int64) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE runtime_state
		SET telegram_offset=GREATEST(telegram_offset,$1), updated_at=now()
		WHERE singleton=true
	`, offset)
	return err
}

func (s *Store) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.pool.Ping(ctx)
}

func (s *Store) HealthStatus(ctx context.Context) (HealthStatus, error) {
	if err := s.pool.Ping(ctx); err != nil {
		return HealthStatus{}, err
	}
	var status HealthStatus
	err := s.pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE status='received'),
			count(*) FILTER (WHERE status='failed')
		FROM conversions
	`).Scan(&status.Received, &status.Failed)
	return status, err
}

func (s *Store) CleanupConversions(ctx context.Context, retention time.Duration) (int64, error) {
	result, err := s.pool.Exec(ctx, `
		DELETE FROM conversions
		WHERE status IN ('sent','failed') AND created_at < now() - $1::interval
	`, interval(retention))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func interval(duration time.Duration) string {
	return fmt.Sprintf("%f seconds", duration.Seconds())
}
