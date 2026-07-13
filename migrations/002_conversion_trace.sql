-- SPDX-License-Identifier: Apache-2.0
-- Private, retention-bound Telegram transport trace for conversion debugging.

ALTER TABLE conversions
  ADD COLUMN IF NOT EXISTS telegram_input JSONB,
  ADD COLUMN IF NOT EXISTS rendered_markdown TEXT,
  ADD COLUMN IF NOT EXISTS telegram_response JSONB;
