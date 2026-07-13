-- SPDX-License-Identifier: Apache-2.0
-- Preserve each Rich Markdown request/result pair for operator debugging.

ALTER TABLE conversions
  ADD COLUMN IF NOT EXISTS telegram_attempts JSONB;
