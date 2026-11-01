// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package toolbackend

import (
	"encoding/json"
	"errors"
)

// Diagnostic is provider evidence, never a permission or a trusted instruction.
// Messages are locally selected; arbitrary remote error text is not propagated.
type Diagnostic struct {
	Code        string `json:"code"`
	Stage       string `json:"stage"`
	Message     string `json:"message"`
	Retryable   bool   `json:"retryable"`
	HTTPStatus  int    `json:"httpStatus,omitempty"`
	LimitBytes  int    `json:"limitBytes,omitempty"`
	ActualBytes int    `json:"actualBytes,omitempty"`
}

type DiagnosticError struct{ Detail Diagnostic }

func (e *DiagnosticError) Error() string {
	if d, ok := KnownDiagnostic(e.Detail.Code); ok {
		return d.Message
	}
	return ErrProvider.Error()
}
func (e *DiagnosticError) Unwrap() error { return ErrProvider }

// KnownDiagnostic defines the bounded wire contract with participating servers.
func KnownDiagnostic(code string) (Diagnostic, bool) {
	entries := map[string][2]string{
		"public-url-refused":        {"input", "The input is not an allowed public HTTPS URL."},
		"url-not-https":             {"input", "The URL must use HTTPS."},
		"url-malformed":             {"input", "The URL is malformed or contains unsupported components."},
		"private-address-refused":   {"input", "The URL resolves to a private or reserved address."},
		"dns-failed":                {"network", "The source hostname could not be resolved."},
		"connect-failed":            {"network", "The connection to the source could not be established."},
		"tls-failed":                {"network", "The source TLS connection could not be verified."},
		"fetch-failed":              {"network", "The source request failed; no more specific cause was reported."},
		"fetch-timeout":             {"network", "The source request exceeded its deadline."},
		"redirect-refused":          {"response", "The source redirected to a target that was refused."},
		"redirect-limit":            {"response", "The source exceeded the redirect limit."},
		"source-unavailable":        {"response", "The source returned an unsuccessful HTTP status."},
		"rate-limited":              {"response", "The source rate limited this request."},
		"search-challenge":          {"response", "The search provider requires an interactive verification."},
		"unsupported-content":       {"response", "The source content type is not supported."},
		"response-too-large":        {"read", "The source response exceeded its byte limit."},
		"body-read-failed":          {"read", "The response body could not be read completely."},
		"parse-failed":              {"extract", "The source body could not be parsed in its declared format."},
		"invalid-encoding":          {"read", "The response body is not valid UTF-8."},
		"invalid-or-large-response": {"read", "The source reported a read, encoding or size failure without distinguishing the cause."},
		"no-usable-content":         {"extract", "The source returned no usable public content."},
		"javascript-required":       {"extract", "The source returned a JavaScript shell rather than usable content."},
		"search-no-results":         {"search", "The search provider returned no usable results."},
		"search-invalid-response":   {"search", "The search provider returned an unsupported result format."},
	}
	pair, ok := entries[code]
	if !ok {
		return Diagnostic{}, false
	}
	retry := code == "fetch-timeout" || code == "connect-failed" || code == "rate-limited"
	return Diagnostic{Code: code, Stage: pair[0], Message: pair[1], Retryable: retry}, true
}

// ParseDiagnostic accepts a small structured envelope, not arbitrary server text.
func ParseDiagnostic(text string) error {
	if len(text) > 2048 {
		return ErrProvider
	}
	var envelope struct {
		Error struct {
			Code        string `json:"code"`
			HTTPStatus  int    `json:"httpStatus"`
			LimitBytes  int    `json:"limitBytes"`
			ActualBytes int    `json:"actualBytes"`
		} `json:"agenovaError"`
	}
	if json.Unmarshal([]byte(text), &envelope) != nil {
		return ErrProvider
	}
	d, ok := KnownDiagnostic(envelope.Error.Code)
	if !ok {
		return ErrProvider
	}
	if n := envelope.Error.HTTPStatus; n >= 100 && n <= 599 {
		d.HTTPStatus = n
		if n >= 500 {
			d.Retryable = true
		}
	}
	if n := envelope.Error.LimitBytes; n > 0 && n <= 8<<20 {
		d.LimitBytes = n
	}
	if n := envelope.Error.ActualBytes; n > 0 && n <= 16<<20 {
		d.ActualBytes = n
	}
	return &DiagnosticError{Detail: d}
}

func ErrorDiagnostic(err error) *Diagnostic {
	var e *DiagnosticError
	if errors.As(err, &e) {
		known, ok := KnownDiagnostic(e.Detail.Code)
		if !ok {
			return nil
		}
		if n := e.Detail.HTTPStatus; n >= 100 && n <= 599 {
			known.HTTPStatus = n
			known.Retryable = known.Retryable || n >= 500
		}
		if n := e.Detail.LimitBytes; n > 0 && n <= 8<<20 {
			known.LimitBytes = n
		}
		if n := e.Detail.ActualBytes; n > 0 && n <= 16<<20 {
			known.ActualBytes = n
		}
		return &known
	}
	return nil
}
