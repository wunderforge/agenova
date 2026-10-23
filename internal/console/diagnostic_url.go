// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"net/url"
	"strings"
)

// SafeDiagnosticURL accepts operator-owned navigation only. It does not fetch
// URLs and never publishes credentials, query tokens or arbitrary relative paths.
func SafeDiagnosticURL(value string) string {
	if value == "/grafana" || value == "/grafana/" {
		return value
	}
	u, err := url.Parse(value)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" || strings.ContainsAny(value, "\r\n\\") {
		return ""
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")) {
		return ""
	}
	return u.String()
}
