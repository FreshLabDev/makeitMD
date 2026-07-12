// SPDX-License-Identifier: Apache-2.0
package db

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/FreshLabDev/makeitMD/internal/telegram"
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

func (s *Store) Touch(ctx context.Context, user telegram.User) error {
	_, err := s.pool.Exec(ctx,
		`SELECT core.touch($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		"makeitmd", user.ID, nullString(user.Username), nullString(user.FirstName),
		nullString(user.LastName), nullString(user.LanguageCode), nil, nil, nil, nil, user.IsBot)
	return err
}

func (s *Store) CreateConversion(ctx context.Context, updateID int64, message telegram.Message) (int64, ConversionStatus, error) {
	var id int64
	var status ConversionStatus
	err := s.pool.QueryRow(ctx, `
		INSERT INTO conversions (
			telegram_update_id, telegram_message_id, telegram_user_id,
			source_text, character_count, byte_count
		) VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (telegram_update_id) DO UPDATE SET telegram_update_id = EXCLUDED.telegram_update_id
		RETURNING id, status
	`, updateID, message.MessageID, message.From.ID, message.Text,
		utf8.RuneCountInString(message.Text), len(message.Text)).Scan(&id, &status)
	return id, status, err
}

func (s *Store) MarkSent(ctx context.Context, id int64) error {
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
		SET status='sent', sent_at=now(), failed_at=NULL, error_code=NULL
		WHERE id=$1 AND status='received'
		RETURNING telegram_user_id, character_count, byte_count, sent_at
	`, id).Scan(&userID, &characters, &bytes, &sentAt)
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

func (s *Store) MarkFailed(ctx context.Context, id int64, errorCode string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE conversions SET status='failed', failed_at=now(), error_code=$2
		WHERE id=$1 AND status='received'
	`, id, errorCode)
	return err
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
