-- SPDX-License-Identifier: Apache-2.0
-- makeitMD domain data in shared core-postgres. Identity remains in core.*.

CREATE TABLE IF NOT EXISTS conversions (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  telegram_update_id BIGINT NOT NULL UNIQUE,
  telegram_message_id BIGINT NOT NULL,
  telegram_user_id BIGINT NOT NULL REFERENCES core.person(telegram_user_id) ON DELETE CASCADE,
  source_text TEXT NOT NULL,
  character_count INTEGER NOT NULL CHECK (character_count >= 0),
  byte_count INTEGER NOT NULL CHECK (byte_count >= 0),
  status TEXT NOT NULL DEFAULT 'received' CHECK (status IN ('received','sent','failed')),
  error_code TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  sent_at TIMESTAMPTZ,
  failed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS ix_conversions_user_created
  ON conversions (telegram_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS ix_conversions_status_created
  ON conversions (status, created_at DESC);

CREATE TABLE IF NOT EXISTS runtime_state (
  singleton BOOLEAN PRIMARY KEY DEFAULT true CHECK (singleton),
  telegram_offset BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO runtime_state(singleton) VALUES (true) ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS user_stats (
  telegram_user_id BIGINT PRIMARY KEY REFERENCES core.person(telegram_user_id) ON DELETE CASCADE,
  conversions BIGINT NOT NULL DEFAULT 0 CHECK (conversions >= 0),
  characters BIGINT NOT NULL DEFAULT 0 CHECK (characters >= 0),
  bytes BIGINT NOT NULL DEFAULT 0 CHECK (bytes >= 0),
  first_conversion_at TIMESTAMPTZ,
  last_conversion_at TIMESTAMPTZ
);
