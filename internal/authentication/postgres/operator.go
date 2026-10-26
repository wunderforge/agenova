// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package postgres

import (
	"context"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"time"
)

// IsOperator selects account-administration navigation. Installation topology
// must independently use AuthorizeInstallation with the serving installation.
func (s Store) IsOperator(ctx context.Context, p v0.Principal) bool {
	if s.DB == nil || p.AuthenticationContext != "oidc" || p.Team == "" || p.Subject == "" {
		return false
	}
	var allowed bool
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	err := s.DB.QueryRowContext(ctx, `SELECT account_role IN ('owner','admin') FROM agenova_members WHERE principal=$1 AND team=$2 AND active`, p.Subject, p.Team).Scan(&allowed)
	return err == nil && allowed
}
