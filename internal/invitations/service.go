// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package invitations coordinates durable membership intent with an external
// directory. It never creates cloud users/roles or accepts a caller's identity.
package invitations

import (
	"context"
	"errors"
	"net/mail"
	"net/url"
	"strings"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
)

var ErrUnavailable = errors.New("invitation delivery is unconfirmed; inspect its status before retrying")
var ErrInvalid = errors.New("provide a valid email, project and role")

type Request struct {
	Email   string `json:"email"`
	Project string `json:"project"`
	Role    string `json:"role"`
}
type Invitation struct {
	LinkManaged   bool      `json:"linkManaged,omitempty"`
	AcceptanceURL string    `json:"acceptanceUrl,omitempty"`
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	Project       string    `json:"project"`
	Role          string    `json:"role"`
	Status        string    `json:"status"`
	ExpiresAt     time.Time `json:"expiresAt"`
	// Private binding never returned to the browser.
	Issuer    string `json:"-"`
	Subject   string `json:"-"`
	Principal string `json:"-"`
	Team      string `json:"-"`
}

func Validate(r Request) bool {
	address, err := mail.ParseAddress(r.Email)
	return err == nil && address.Address == r.Email && address.Name == "" && len(r.Email) <= 254 && strings.TrimSpace(r.Email) == r.Email && r.Email == strings.ToLower(r.Email) && r.Project != "" && len(r.Project) <= 128 && (r.Role == "admin" || r.Role == "member" || r.Role == "viewer")
}

// Create reserves a directory identity without sending mail. Send is explicit;
// an uncertain external result is never automatically retried by this service.
type Directory interface {
	Create(context.Context, string) (string, error)
	Send(context.Context, string) error
}
type Store interface {
	Reserve(context.Context, v0.Principal, string, Request) (Invitation, error)
	Bind(context.Context, v0.Principal, string, string) (Invitation, error)
	Delivered(context.Context, string) error
	Uncertain(context.Context, string) error
	ListInvitations(context.Context, v0.Principal) ([]Invitation, error)
}
type Service struct {
	Google    bool
	Origin    string
	Issuer    string
	Directory Directory
	Store     Store
}

func (s Service) Invite(ctx context.Context, actor v0.Principal, r Request) (Invitation, error) {
	if !Validate(r) {
		return Invitation{}, ErrInvalid
	}
	if s.Google {
		store, ok := s.Store.(interface {
			ReserveGoogleInvitation(context.Context, v0.Principal, Request) (Invitation, string, error)
		})
		origin, err := url.Parse(s.Origin)
		if !ok || err != nil || origin.Scheme != "https" || origin.Host == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
			return Invitation{}, ErrUnavailable
		}
		invite, proof, err := store.ReserveGoogleInvitation(ctx, actor, r)
		if err != nil {
			return invite, err
		}
		invite.AcceptanceURL = s.Origin + "/login#invite=" + proof
		return invite, nil
	}
	if s.Issuer == "" || s.Directory == nil || s.Store == nil {
		return Invitation{}, ErrUnavailable
	}
	invite, err := s.Store.Reserve(ctx, actor, s.Issuer, r)
	if err != nil {
		return Invitation{}, err
	}
	subject, err := s.Directory.Create(ctx, r.Email)
	if err != nil || subject == "" {
		_ = s.Store.Uncertain(context.WithoutCancel(ctx), invite.ID)
		return invite, ErrUnavailable
	}
	reservedID := invite.ID
	invite, err = s.Store.Bind(ctx, actor, invite.ID, subject)
	if err != nil {
		_ = s.Store.Uncertain(context.WithoutCancel(ctx), reservedID)
		return Invitation{}, ErrUnavailable
	}
	if s.Directory.Send(ctx, subject) != nil {
		_ = s.Store.Uncertain(context.WithoutCancel(ctx), invite.ID)
		return invite, ErrUnavailable
	}
	if s.Store.Delivered(ctx, invite.ID) != nil {
		return invite, ErrUnavailable
	}
	invite.Status = "sent"
	return invite, nil
}

// BootstrapOwner is reachable only from the privileged operator command. The
// database refuses a second bootstrap, including while the first is pending.
func (s Service) BootstrapOwner(ctx context.Context, team string, r Request) (Invitation, error) {
	store, ok := s.Store.(interface {
		ReserveOwner(context.Context, string, string, Request) (Invitation, error)
		BindOwner(context.Context, string, string, string) (Invitation, error)
	})
	if !ok || !Validate(r) || r.Role != "admin" || s.Directory == nil || s.Issuer == "" {
		return Invitation{}, ErrInvalid
	}
	invite, err := store.ReserveOwner(ctx, s.Issuer, team, r)
	if err != nil {
		return Invitation{}, err
	}
	subject, err := s.Directory.Create(ctx, r.Email)
	if err != nil || subject == "" {
		_ = s.Store.Uncertain(context.WithoutCancel(ctx), invite.ID)
		return invite, ErrUnavailable
	}
	reservedID := invite.ID
	invite, err = store.BindOwner(ctx, team, invite.ID, subject)
	if err != nil {
		_ = s.Store.Uncertain(context.WithoutCancel(ctx), reservedID)
		return Invitation{}, ErrUnavailable
	}
	if s.Directory.Send(ctx, subject) != nil {
		_ = s.Store.Uncertain(context.WithoutCancel(ctx), invite.ID)
		return invite, ErrUnavailable
	}
	if s.Store.Delivered(ctx, invite.ID) != nil {
		return invite, ErrUnavailable
	}
	invite.Status = "sent"
	return invite, nil
}
