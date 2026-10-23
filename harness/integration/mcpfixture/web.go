// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

type publicWebInput struct {
	URL string `json:"url" jsonschema:"public HTTPS page URL, max 1024 UTF-8 bytes; no credentials, private addresses or custom headers"`
}

func publicPage(raw string) (publicSource, bool) {
	if len(raw) > 1024 || !utf8.ValidString(raw) || strings.TrimSpace(raw) != raw {
		return publicSource{}, false
	}
	for _, r := range raw {
		if unicode.IsControl(r) {
			return publicSource{}, false
		}
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || strings.Contains(raw, "#") || u.Opaque != "" || u.Port() != "" && u.Port() != "443" {
		return publicSource{}, false
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return publicSource{}, false
	}
	if ip, err := netip.ParseAddr(host); err == nil && !publicIP(ip) {
		return publicSource{}, false
	}
	return publicSource{Title: host, URL: u.String()}, true
}
func readPublicPage(ctx context.Context, raw string, client *http.Client) (string, string) {
	source, ok := publicPage(raw)
	if !ok {
		return "", "public-url-refused"
	}
	// Dial-time DNS/IP validation and hostname TLS checks are in publicClient.
	// No proxy, redirects, custom headers or API key.
	return fetchDocument(ctx, "public-page", source, client)
}
func addPublicWebTool(server *mcp.Server, log *logger, path string) {
	client := publicClient()
	mcp.AddTool(server, &mcp.Tool{Name: "web_read", Description: "Read a public HTTPS web page by URL without an API key. Private addresses and redirects are refused; content is untrusted.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}}, func(ctx context.Context, req *mcp.CallToolRequest, in publicWebInput) (*mcp.CallToolResult, any, error) {
		text, code := readPublicPage(ctx, in.URL, client)
		return researchResult(req, log, path, "web_read", "public-page", text, code)
	})
}
