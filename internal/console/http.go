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
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/identity"
	"github.com/wunderforge/agenova/internal/policy"
)

const maxSubmissionBytes = 128 << 10

// Setup describes operator configuration, not a live health assessment.
type Setup struct {
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
func Handler(service *Service) http.Handler {
	return handler(service, false)
}

type verifiedPrincipalKey struct{}

// AuthenticatedHandler protects the ordinary Work/evidence surface with one
// verified bearer identity. It is a separate composition from fixed-principal
// Handler, so authentication failure can never fall back to reference mode.
func AuthenticatedHandler(service *Service, provider identity.Provider) http.Handler {
	base := handler(service, true)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if provider == nil {
			writeError(w, http.StatusServiceUnavailable, "authentication_unavailable", "Authentication is unavailable.")
			return
		}
		token, ok := bearerToken(r.Header.Values("Authorization"))
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication_failed", "A valid bearer credential is required.")
			return
		}
		principal, err := provider.Verify(r.Context(), token)
		if err != nil || principal.Validate() != nil {
			writeError(w, http.StatusUnauthorized, "authentication_failed", "A valid bearer credential is required.")
			return
		}
		base.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), verifiedPrincipalKey{}, principal)))
	})
}

func handler(service *Service, authenticated bool) http.Handler {
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
		var verified *identity.VerifiedPrincipal
		if authenticated {
			principal, ok := r.Context().Value(verifiedPrincipalKey{}).(identity.VerifiedPrincipal)
			if !ok || principal.Validate() != nil {
				writeError(w, http.StatusUnauthorized, "authentication_failed", "A valid bearer credential is required.")
				return
			}
			verified = &principal
		}
		switch r.URL.Path {
		case "/api/setup":
			if !requireMethod(w, r, http.MethodGet) {
				return
			}
			setup, err := service.setup()
			if err != nil {
				writeError(w, 503, "setup_unavailable", "Registered platform setup is unavailable.")
				return
			}
			// An empty default-deny policy is valid. Keep the HTTP collection
			// shape stable even for older records persisted with a nil Go slice.
			if setup.Policy.Rules == nil {
				setup.Policy.Rules = []policy.Rule{}
			}
			if verified != nil {
				setup.Principal = verified.Principal()
			}
			writeJSON(w, 200, setup)
		case "/api/requests":
			switch r.Method {
			case http.MethodGet:
				if verified == nil {
					writeJSON(w, 200, service.List())
				} else {
					writeJSON(w, 200, service.ListVerified(*verified))
				}
			case http.MethodPost:
				submitHTTP(w, r, service, verified)
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
				view, err := service.QueryRequest(ref)
				if verified != nil {
					view, err = service.QueryRequestVerified(ref, *verified)
				}
				if route.claim && verified == nil {
					view, err = service.QueryClaim(ref)
				} else if route.claim {
					view, err = service.QueryClaimVerified(ref, *verified)
				}
				if err != nil {
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

func submitHTTP(w http.ResponseWriter, r *http.Request, service *Service, principal *identity.VerifiedPrincipal) {
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
	if principal == nil {
		view, err = service.Submit(data)
	} else {
		view, err = service.SubmitVerified(data, *principal)
	}
	if err != nil {
		var submission *SubmissionError
		switch {
		case errors.Is(err, ErrConflict):
			writeError(w, 409, "request_conflict", "This request reference already exists.")
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

func bearerToken(values []string) (string, bool) {
	if len(values) != 1 {
		return "", false
	}
	parts := strings.Fields(values[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" || strings.ContainsAny(parts[1], "\r\n") {
		return "", false
	}
	return parts[1], true
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
