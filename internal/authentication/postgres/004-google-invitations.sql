-- Copyright 2026 Agenova contributors.
-- SPDX-License-Identifier: Apache-2.0
ALTER TABLE agenova_invitations ADD COLUMN acceptance_hash text;
CREATE UNIQUE INDEX agenova_invitation_acceptance_hash ON agenova_invitations(acceptance_hash) WHERE acceptance_hash IS NOT NULL;
