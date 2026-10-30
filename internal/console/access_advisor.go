// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/accessadvisor"
	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/toolbackend"
	"github.com/wunderforge/agenova/internal/workstore"
)

type adviceInput struct {
	Mode    string           `json:"mode,omitempty"`
	Request *v0.ClaimRequest `json:"request"`
}

func advisorError(w http.ResponseWriter, err error) {
	var a *accessadvisor.Error
	var sub *SubmissionError
	var invalid *v0.ValidationError
	switch {
	case errors.Is(err, authentication.ErrForbidden):
		writeError(w, 403, "access_denied", "This project operation is not permitted.")
	case errors.Is(err, ErrConflict), errors.Is(err, workstore.ErrConflict):
		writeError(w, 409, "request_conflict", "This request name belongs to a different submission.")
	case errors.Is(err, workstore.ErrUnavailable):
		writeError(w, 503, "storage_unavailable", "Work storage is unavailable; do not resubmit with a new name.")
	case errors.As(err, &sub):
		writeError(w, 422, sub.Code, sub.Message)
	case errors.As(err, &invalid):
		writeError(w, 422, "authority_request_rejected", "The proposed permissions do not fit current Policy or template limits; no Work was started.")
	case errors.As(err, &a):
		status := 422
		switch a.Code {
		case "advice_conflict":
			status = 409
		case "advice_not_found":
			status = 404
		case "advisor_disabled", "mode_disabled":
			status = 403
		case "advisor_busy", "advisor_budget", "advice_pending":
			status = 429
		case "advisor_failed":
			status = 503
		}
		writeError(w, status, a.Code, a.Message)
	default:
		writeError(w, 503, "advisor_unavailable", "The result is not confirmed. Check the existing request before retrying.")
	}
}

func (s *Service) adviceContext(ctx context.Context, p v0.Principal, r *v0.ClaimRequest, check func(context.Context) error) (Setup, error) {
	if p.Subject == "" || p.Team == "" || p.AuthenticationContext != "oidc" || s.setupFor == nil || s.access == nil {
		return Setup{}, authentication.ErrForbidden
	}
	if r == nil || v0.ValidateClaimRequest(r) != nil || !validReference(r.Metadata.Name) || len(r.Metadata.Name) > 256 {
		return Setup{}, &accessadvisor.Error{Code: "invalid_task", Message: "Provide a canonical bounded task request."}
	}
	if check == nil || check(ctx) != nil || s.access.Authorize(ctx, p, "work.submit", r.Spec.ProjectRef) != nil {
		return Setup{}, authentication.ErrForbidden
	}
	setup, e := s.setupFor(ctx, p, r.Spec.ProjectRef, r.Spec.TemplateRef)
	if e != nil || setup.Template == nil || setup.Project != r.Spec.ProjectRef || setup.Template.Metadata.Name != r.Spec.TemplateRef {
		return Setup{}, authentication.ErrForbidden
	}
	return setup, nil
}

func (s *Service) propose(ctx context.Context, p v0.Principal, r *v0.ClaimRequest, check func(context.Context) error) (accessadvisor.Plan, error) {
	setup, e := s.adviceContext(ctx, p, r, check)
	if e != nil {
		return accessadvisor.Plan{}, e
	}
	var catalog []toolbackend.Descriptor
	if s.tools != nil {
		var scopes []string
		if setup.Template.Spec.AccessGuide != nil {
			for _, profile := range setup.Template.Spec.AccessGuide.Profiles {
				scopes = append(scopes, profile.RequestedAccess.ResourceScopes...)
			}
		}
		catalog = s.tools.Catalog().Intersect(setup.Template.Spec.CapabilityCeiling.Tools, scopes).Entries()
	}
	plan, e := s.advisor.Plan(ctx, p, r, setup.Template, catalog)
	if check(ctx) != nil || s.access.Authorize(ctx, p, "work.submit", r.Spec.ProjectRef) != nil {
		return accessadvisor.Plan{}, authentication.ErrForbidden
	}
	return plan, e
}

func (s *Service) acceptAdvice(ctx context.Context, p v0.Principal, r *v0.ClaimRequest, id, mode string, check func(context.Context) error) (*evidence.AccessAdvice, error) {
	setup, e := s.adviceContext(ctx, p, r, check)
	if e != nil {
		return nil, e
	}
	plan, e := s.advisor.Get(p, id)
	if e != nil {
		return nil, e
	}
	if plan.Status != "ready" || plan.Request == nil || plan.IntentDigest != accessadvisor.IntentDigest(r) || plan.GuideDigest != v0.AccessGuideDigest(setup.Template) {
		return nil, &accessadvisor.Error{Code: "advice_stale", Message: "Task, template or advice changed; plan again explicitly."}
	}
	receipt := &evidence.AccessAdvice{ID: plan.ID, Mode: mode, Engine: plan.Engine, IntentDigest: plan.IntentDigest, GuideDigest: plan.GuideDigest, GuideVersion: plan.GuideVersion, ProfileID: plan.ProfileID, AcceptedRequestDigest: accessadvisor.Digest(r)}
	if plan.Model != nil {
		receipt.PlannerModel = plan.Model.Name
		receipt.PlannerInputTokens = plan.Model.InputTokens
		receipt.PlannerOutputTokens = plan.Model.OutputTokens
	}
	return receipt, nil
}

func advisorHTTP(w http.ResponseWriter, r *http.Request, s *Service, p v0.Principal, check func(context.Context) error) {
	if p.Subject == "" || check == nil {
		writeError(w, 401, "viewer_required", "Sign in to request access advice.")
		return
	}
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/access-advice/") {
		id := strings.TrimPrefix(r.URL.Path, "/api/access-advice/")
		plan, e := s.advisor.Get(p, id)
		if e != nil {
			advisorError(w, e)
			return
		}
		if check(r.Context()) != nil || s.access == nil || s.access.Authorize(r.Context(), p, "work.submit", plan.Project) != nil {
			advisorError(w, authentication.ErrForbidden)
			return
		}
		if plan.Request != nil {
			if _, e = s.adviceContext(r.Context(), p, plan.Request, check); e != nil {
				advisorError(w, e)
				return
			}
		} else if check(r.Context()) != nil {
			advisorError(w, authentication.ErrForbidden)
			return
		}
		writeJSON(w, 200, plan)
		return
	}
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	media, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || media != "application/json" {
		writeError(w, 415, "unsupported_media_type", "Submit application/json.")
		return
	}
	var in adviceInput
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxSubmissionBytes))
	d.DisallowUnknownFields()
	if d.Decode(&in) != nil || d.Decode(new(any)) != io.EOF {
		writeError(w, 400, "invalid_request", "Provide only mode and a canonical task request.")
		return
	}
	if in.Mode == "" {
		in.Mode = s.advisor.Info().DefaultMode
	}
	automatic := r.URL.Path == "/api/access-submit"
	expected := "suggest"
	if automatic {
		expected = "automatic"
	}
	if in.Mode != expected || !s.advisor.Allows(in.Mode) {
		advisorError(w, &accessadvisor.Error{Code: "mode_disabled", Message: "This access planning mode is not enabled; explicit Work submission remains separate."})
		return
	}
	if _, e = s.adviceContext(r.Context(), p, in.Request, check); e != nil {
		advisorError(w, e)
		return
	}
	if accessadvisor.HasAccess(in.Request.Spec.RequestedAccess) {
		advisorError(w, &accessadvisor.Error{Code: "explicit_access", Message: "Submit explicit permissions through the normal Work endpoint."})
		return
	}
	if automatic {
		// Recover existing canonical Work before spending on inference, including
		// after restart when ephemeral draft cache no longer exists.
		ref := in.Request.Metadata.Name
		if s.durable != nil {
			ref = workstore.Reference(p, ref)
		}
		prior, lookupErr := s.QueryFor(p, ref, false)
		if errors.Is(lookupErr, workstore.ErrUnavailable) {
			advisorError(w, lookupErr)
			return
		}
		if lookupErr == nil {
			if prior.AccessAdvice == nil || prior.AccessAdvice.Mode != "automatic" || prior.AccessAdvice.IntentDigest != accessadvisor.IntentDigest(in.Request) {
				advisorError(w, ErrConflict)
				return
			}
			writeJSON(w, 200, prior)
			return
		}
	}
	plan, e := s.propose(r.Context(), p, in.Request, check)
	if e != nil {
		advisorError(w, e)
		return
	}
	if !automatic || plan.Status != "ready" {
		writeJSON(w, 200, plan)
		return
	}
	receipt, e := s.acceptAdvice(r.Context(), p, plan.Request, plan.ID, "automatic", check)
	if e != nil {
		advisorError(w, e)
		return
	}
	data, e := json.Marshal(plan.Request)
	if e != nil {
		advisorError(w, e)
		return
	}
	view, e := s.submitForSessionPlanned(data, p, check, receipt)
	if e != nil {
		advisorError(w, e)
		return
	}
	writeJSON(w, 202, view)
}
