-- Copyright 2026 Agenova contributors.
-- SPDX-License-Identifier: Apache-2.0

-- Existing attempts remain pending and expire naturally within five minutes.
ALTER TABLE agenova_login_attempts
 ADD COLUMN attempt_kind text NOT NULL DEFAULT 'pending' CHECK (attempt_kind IN ('pending', 'verified')),
 ADD COLUMN source_hash text NOT NULL DEFAULT '';
CREATE INDEX agenova_login_attempts_capacity ON agenova_login_attempts(attempt_kind, source_hash);
