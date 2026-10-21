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
	"net/url"
	"strings"
	"unicode"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/history"
	"github.com/wunderforge/agenova/internal/policy"
	"github.com/wunderforge/agenova/internal/workstore"
	"strconv"
)

const maxSubmissionBytes = 128 << 10

// Setup describes operator configuration, not a live health assessment.
type Setup struct {
	Project      string               `json:"project,omitempty"`
	Projects     []string             `json:"projects,omitempty"`
	Templates    []string             `json:"templates,omitempty"`
	Principal    v0.Principal         `json:"principal"`
	Template     *v0.AgentTemplate    `json:"template"`
	Policy       policy.PolicyBundle  `json:"policy"`
	Capabilities map[string]string    `json:"capabilities"`
	Installation InstallationIdentity `json:"installation"`
}

type InstallationIdentity struct {
	Kind     string `json:"kind"`
	Platform string `json:"platform,omitempty"`
	Revision string `json:"revision,omitempty"`
}

type httpError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Handler exposes only the approved bounded local console surface. It never
// accepts caller identity, effective authority, or operator configuration.
func Handler(service *Service) http.Handler { return handler(service, nil) }

// Authenticator resolves verified identity and current membership on every request.
type Authenticator interface {
	Authenticate(*http.Request) (v0.Principal, error)
}

type SessionAuthenticator interface {
	Authenticator
	AuthenticateSession(*http.Request) (v0.Principal, func(context.Context) error, error)
}

func AuthenticatedHandler(service *Service, auth SessionAuthenticator) (http.Handler, error) {
	if service == nil || service.prepareFor == nil || service.access == nil || auth == nil {
		return nil, errors.New("authenticated service and identity boundary required")
	}
	return handler(service, auth), nil
}

func handler(service *Service, auth Authenticator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if !sameOrigin(r) {
			writeError(w, http.StatusForbidden, "origin_rejected", "Use the same-origin console.")
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "unavailable", "The console service is unavailable.")
			return
		}
		var principal v0.Principal
		var sessionCheck func(context.Context) error
		if service.prepareFor != nil && auth == nil {
			writeError(w, 503, "authentication_required", "Authenticated service configuration is required.")
			return
		}
		if auth != nil {
			var err error
			if sessionAuth, ok := auth.(SessionAuthenticator); ok {
				principal, sessionCheck, err = sessionAuth.AuthenticateSession(r)
				if sessionCheck == nil && err == nil {
					err = authentication.ErrForbidden
				}
			} else {
				err = authentication.ErrForbidden
			}
			if err != nil {
				status := http.StatusUnauthorized
				if errors.Is(err, authentication.ErrForbidden) {
					status = http.StatusForbidden
				}
				writeError(w, status, "access_denied", "Sign in with an active membership.")
				return
			}
		}
		switch r.URL.Path {
		case "/api/provider-observations":
			if auth != nil {
				writeError(w, 403, "access_denied", "Operator diagnostics are not a member capability.")
				return
			}
			if !requireMethod(w, r, http.MethodGet) {
				return
			}
			counter, ok := service.provider.(interface{ InvocationCount() uint64 })
			if !ok {
				writeError(w, 501, "unsupported", "Provider invocation observations are unavailable.")
				return
			}
			writeJSON(w, 200, map[string]any{"scope": "current-service-process", "providerInvocationAttempts": counter.InvocationCount()})
		case "/api/setup":
			if !requireMethod(w, r, http.MethodGet) {
				return
			}
			var setup Setup
			var err error
			if auth != nil && service.setupFor != nil {
				setup, err = service.setupFor(r.Context(), principal, r.URL.Query().Get("project"), r.URL.Query().Get("template"))
			} else {
				setup, err = service.setup()
			}
			if err != nil {
				writeError(w, 503, "setup_unavailable", "Registered platform setup is unavailable.")
				return
			}
			if auth != nil {
				setup.Principal = principal
				if service.setupFor == nil {
					rules := []policy.Rule{}
					for _, rule := range setup.Policy.Rules {
						if rule.Team == principal.Team && service.access.Authorize(r.Context(), principal, "work.submit", rule.Project) == nil {
							rules = append(rules, rule)
						}
					}
					setup.Policy.Rules = rules
					visible := false
					for _, rule := range rules {
						if setup.Template != nil && rule.Action == "claim.create" && rule.TemplateRef == setup.Template.Metadata.Name {
							visible = true
						}
					}
					if !visible {
						setup.Template = nil
					}
				}
			}
			// An empty default-deny policy is valid. Keep the HTTP collection
			// shape stable even for older records persisted with a nil Go slice.
			if setup.Policy.Rules == nil {
				setup.Policy.Rules = []policy.Rule{}
			}
			writeJSON(w, 200, setup)
		case "/api/history":
			if !requireMethod(w, r, http.MethodGet) {
				return
			}
			if auth == nil {
				writeError(w, 401, "viewer_required", "Verified viewer required.")
				return
			}
			limit := 20
			var err error
			if raw := r.URL.Query().Get("limit"); raw != "" {
				limit, err = strconv.Atoi(raw)
			}
			if err != nil || limit < 1 || limit > 100 {
				writeError(w, 400, "invalid_cursor", "Invalid history page.")
				return
			}
			var page history.Page
			if source, ok := service.durable.(interface {
				HistoryFor(context.Context, v0.Principal, int, string) (history.Page, error)
			}); ok && !service.storageFailed.Load() {
				page, err = source.HistoryFor(r.Context(), principal, limit, r.URL.Query().Get("cursor"))
			} else {
				err = history.ErrUnavailable
			}
			if err != nil {
				if errors.Is(err, history.ErrCursor) {
					writeError(w, 400, "invalid_cursor", "Invalid history page.")
				} else {
					writeError(w, 503, "history_unavailable", "History is unavailable.")
				}
				return
			}
			writeJSON(w, 200, page)
		case "/api/requests":
			switch r.Method {
			case http.MethodGet:
				if auth != nil {
					views, err := service.ListForResult(principal)
					if err != nil {
						writeError(w, 503, "evidence_unavailable", "Durable evidence is unavailable.")
						return
					}
					writeJSON(w, 200, views)
				} else {
					writeJSON(w, 200, service.List())
				}
			case http.MethodPost:
				submitHTTP(w, r, service, principal, sessionCheck)
			default:
				w.Header().Set("Allow", "GET, POST")
				writeError(w, 405, "method_not_allowed", "This method is not supported.")
			}
		default:
			for _, route := range []struct {
				prefix string
				claim  bool
			}{{"/api/requests/", false}, {"/api/claims/", true}} {
				if !strings.HasPrefix(r.URL.Path, route.prefix) {
					continue
				}
				if !requireMethod(w, r, http.MethodGet) {
					return
				}
				if !strings.HasSuffix(r.URL.Path, "/evidence") {
					writeError(w, 404, "not_found", "The record was not found.")
					return
				}
				ref := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, route.prefix), "/evidence")
				if !validReference(ref) {
					writeError(w, 400, "invalid_reference", "Provide a bounded record reference.")
					return
				}
				var view evidence.View
				var err error
				if auth != nil {
					view, err = service.QueryFor(principal, ref, route.claim)
				} else {
					view, err = service.QueryRequest(ref)
					if route.claim {
						view, err = service.QueryClaim(ref)
					}
				}
				if err != nil {
					if errors.Is(err, workstore.ErrUnavailable) {
						writeError(w, 503, "storage_unavailable", "Work storage is unavailable.")
						return
					}
					writeError(w, 404, "not_found", "The record was not found.")
					return
				}
				writeJSON(w, 200, view)
				return
			}
			writeError(w, 404, "not_found", "The record was not found.")
		}
	})
}

func submitHTTP(w http.ResponseWriter, r *http.Request, service *Service, principal v0.Principal, sessionCheck func(context.Context) error) {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		writeError(w, 415, "unsupported_media_type", "Submit application/json.")
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxSubmissionBytes))
	if err != nil {
		var oversized *http.MaxBytesError
		if errors.As(err, &oversized) {
			writeError(w, 413, "submission_too_large", "The submission exceeds the size limit.")
		} else {
			writeError(w, 400, "invalid_request", "Submit a valid ClaimRequest.")
		}
		return
	}
	request, validationErr := v0.ParseClaimRequestJSON(data)
	if validationErr != nil {
		writeError(w, 400, "invalid_request", "Submit a valid ClaimRequest.")
		return
	}
	objective, ok := request.Spec.Task.Input["objective"].(string)
	if !validReference(request.Metadata.Name) || len(request.Metadata.Name) > 256 || !ok || strings.TrimSpace(objective) == "" || len(objective) > 64<<10 {
		writeError(w, 400, "invalid_request", "Submit a bounded task objective and request reference.")
		return
	}
	var view evidence.View
	if principal.Subject != "" {
		view, err = service.submitForSession(data, principal, sessionCheck)
	} else {
		view, err = service.Submit(data)
	}
	if err != nil {
		var submission *SubmissionError
		switch {
		case errors.Is(err, authentication.ErrForbidden):
			writeError(w, 403, "access_denied", "This project operation is not permitted.")
		case errors.Is(err, ErrConflict), errors.Is(err, workstore.ErrConflict):
			writeError(w, 409, "request_conflict", "This request reference already exists.")
		case errors.Is(err, workstore.ErrUnavailable):
			writeError(w, 503, "storage_unavailable", "Work storage is unavailable. Retry with the same request name.")
		case errors.Is(err, ErrCapacity):
			writeError(w, 429, "capacity_reached", "The local console has reached its record limit.")
		case errors.As(err, &submission):
			writeError(w, 422, submission.Code, submission.Message)
		default:
			writeError(w, 500, "submission_failed", "Submission failed. Check the active policy, registered AgentTemplate, installed profiles and gateway capabilities.")
		}
		return
	}
	status := http.StatusOK
	if view.State != nil && view.State.Decision.Result == v0.DecisionResultAllow {
		status = http.StatusAccepted
	}
	writeJSON(w, status, view)
}

func sameOrigin(r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	origins, present := r.Header["Origin"]
	if !present {
		return true
	} // CLI and other non-browser clients.
	if len(origins) != 1 || origins[0] == "" {
		return false
	}
	u, err := url.Parse(origins[0])
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return err == nil && u.Scheme == scheme && u.Host == r.Host && u.User == nil && u.Path == "" && u.RawQuery == "" && !u.ForceQuery && !strings.Contains(origins[0], "#") && u.Opaque == ""
}

func validReference(ref string) bool {
	// System-issued claim IDs include request/issuance prefixes, so their bound
	// must exceed the independently bounded 256-byte request name.
	if strings.TrimSpace(ref) == "" || len(ref) > 1024 || strings.ContainsAny(ref, "/\\") {
		return false
	}
	for _, ch := range ref {
		if unicode.IsControl(ch) {
			return false
		}
	}
	return true
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	w.Header().Set("Allow", method)
	writeError(w, 405, "method_not_allowed", "This method is not supported.")
	return false
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, httpError{Code: code, Message: message})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		status = 500
		data = []byte(`{"code":"response_failed","message":"The response is unavailable."}`)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(append(data, '\n'))
}
