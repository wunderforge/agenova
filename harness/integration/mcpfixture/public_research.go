// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/net/html"
)

var errSourceDNS = errors.New("public source DNS failed")
var errSourcePrivate = errors.New("public source address refused")
var errSourceRedirect = errors.New("public source redirect refused")
var errSourceRedirectLimit = errors.New("public source redirect limit")

func sourceError(err error) string {
	if errors.Is(err, errSourcePrivate) {
		return "private-address-refused"
	}
	if errors.Is(err, errSourceDNS) {
		return "dns-failed"
	}
	if errors.Is(err, errSourceRedirect) {
		return "redirect-refused"
	}
	if errors.Is(err, errSourceRedirectLimit) {
		return "redirect-limit"
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		return "dns-failed"
	}
	var certificate *tls.CertificateVerificationError
	var unknown x509.UnknownAuthorityError
	if errors.As(err, &certificate) || errors.As(err, &unknown) {
		return "tls-failed"
	}
	var netError net.Error
	if errors.As(err, &netError) && netError.Timeout() {
		return "fetch-timeout"
	}
	var op *net.OpError
	if errors.As(err, &op) {
		return "connect-failed"
	}
	return "fetch-failed"
}

func requestPublic(ctx context.Context, address string, client *http.Client) ([]byte, string, string, string) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, "", "", "url-malformed"
	}
	request.Header.Set("Accept", "text/html,text/plain,application/json,application/rss+xml,application/xml")
	request.Header.Set("User-Agent", "Agenova-Research-Demo/1.0")
	response, err := client.Do(request)
	if err != nil {
		return nil, "", "", sourceError(err)
	}
	defer response.Body.Close()
	if response.StatusCode == 429 {
		return nil, "", "", "rate-limited"
	}
	if response.StatusCode != 200 {
		return nil, "", "", "http-status-" + strconv.Itoa(response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, (2<<20)+1))
	if err != nil {
		return nil, "", "", "body-read-failed"
	}
	if len(body) > 2<<20 {
		return nil, "", "", "response-too-large"
	}
	if !utf8.Valid(body) {
		return nil, "", "", "invalid-encoding"
	}
	finalURL := address
	if response.Request != nil && response.Request.URL != nil {
		finalURL = response.Request.URL.String()
	}
	return body, response.Header.Get("Content-Type"), finalURL, ""
}

func readableHTML(raw []byte) string {
	root, err := html.Parse(strings.NewReader(string(raw)))
	if err != nil {
		return ""
	}
	var chosen *html.Node
	var find func(*html.Node)
	find = func(n *html.Node) {
		if chosen != nil {
			return
		}
		if n.Type == html.ElementNode {
			match := n.Data == "main" || n.Data == "article"
			for _, a := range n.Attr {
				match = match || a.Key == "role" && a.Val == "main" || a.Key == "id" && (a.Val == "content" || a.Val == "main-content")
			}
			if match {
				chosen = n
				return
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			find(child)
		}
	}
	find(root)
	if chosen == nil {
		chosen = root
	}
	var chunks []string
	var collect func(*html.Node)
	collect = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "nav", "header", "footer", "noscript", "form", "button", "svg":
				return
			}
			for _, a := range n.Attr {
				if a.Key == "hidden" || a.Key == "aria-hidden" && a.Val == "true" {
					return
				}
			}
		}
		if n.Type == html.TextNode {
			chunks = append(chunks, n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			collect(child)
		}
	}
	collect(chosen)
	return strings.Join(strings.Fields(strings.Join(chunks, " ")), " ")
}

func readDocument(ctx context.Context, id string, source publicSource, client *http.Client) (string, string) {
	body, kind, finalURL, code := requestPublic(ctx, source.URL, client)
	if code != "" {
		return "", code
	}
	text := ""
	switch {
	case strings.HasPrefix(kind, "text/html"):
		text = readableHTML(body)
	case strings.HasPrefix(kind, "text/plain"):
		text = strings.TrimSpace(string(body))
	case strings.HasPrefix(kind, "application/json"):
		var content any
		if json.Unmarshal(body, &content) != nil {
			return "", "parse-failed"
		}
		compact, _ := json.Marshal(content)
		text = string(compact)
	default:
		return "", "unsupported-content"
	}
	if len(strings.Fields(text)) < 25 {
		lower := strings.ToLower(text)
		if strings.Contains(lower, "javascript") || strings.Contains(lower, "enable scripts") {
			return "", "javascript-required"
		}
	}
	if len(text) < 20 {
		return "", "no-usable-content"
	}
	budget := 2000
	if json.Valid([]byte(text)) {
		budget = 1200
		var compact bytes.Buffer
		if json.Compact(&compact, []byte(text)) == nil {
			text = compact.String()
		}
	}
	truncated := len(text) > budget
	if truncated {
		text = text[:budget]
		for !utf8.ValidString(text) {
			text = text[:len(text)-1]
		}
	}
	value := map[string]any{"source_id": id, "title": source.Title, "url": finalURL, "fetched_at": time.Now().UTC().Format(time.RFC3339), "content": text, "truncated": truncated, "trust": "untrusted public source"}
	result, _ := json.Marshal(value)
	return string(result), ""
}

type searchInput struct {
	Query string `json:"query" jsonschema:"public web search query, not a URL; max 512 bytes"`
}
type searchResult struct {
	Title   string `xml:"title" json:"title"`
	URL     string `xml:"link" json:"url"`
	Snippet string `xml:"description" json:"snippet"`
}
type searchProvider interface {
	Search(context.Context, string) ([]searchResult, string)
}
type rssSearchProvider struct {
	client   *http.Client
	endpoint string
}

func (p rssSearchProvider) Search(ctx context.Context, query string) ([]searchResult, string) {
	if strings.TrimSpace(query) == "" || len(query) > 512 || strings.ContainsAny(query, "\x00\r\n") {
		return nil, "url-malformed"
	}
	address := p.endpoint + "?" + url.Values{"q": {query}, "format": {"rss"}}.Encode()
	body, _, _, code := requestPublic(ctx, address, p.client)
	if code != "" {
		return nil, code
	}
	lower := strings.ToLower(string(body))
	if strings.Contains(lower, "captcha") || strings.Contains(lower, "challenge-form") {
		return nil, "search-challenge"
	}
	var document struct {
		Channel struct {
			Items []searchResult `xml:"item"`
		} `xml:"channel"`
	}
	if xml.Unmarshal(body, &document) != nil {
		return nil, "search-invalid-response"
	}
	rows := []searchResult{}
	for _, r := range document.Channel.Items {
		if _, ok := publicPage(r.URL); !ok {
			continue
		}
		r.Title = strings.Join(strings.Fields(r.Title), " ")
		r.Snippet = readableHTML([]byte(r.Snippet))
		if len(r.Title) > 180 {
			r.Title = utf8Prefix(r.Title, 180)
		}
		if len(r.Snippet) > 450 {
			r.Snippet = utf8Prefix(r.Snippet, 450)
		}
		rows = append(rows, r)
		if len(rows) == 3 {
			break
		}
	}
	if len(rows) == 0 {
		return nil, "search-no-results"
	}
	return rows, ""
}
func addSearchTool(server *mcp.Server, log *logger, path string) {
	// Host-selected provider, never a worker URL. The provider boundary can be
	// replaced without changing Work authority or the logical search contract.
	provider := rssSearchProvider{client: publicClient(), endpoint: "https://www.bing.com/search"}
	mcp.AddTool(server, &mcp.Tool{Name: "web_search", Description: "Search actual public web sources. Returns titles, URLs and snippets, not full pages.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}}, func(ctx context.Context, request *mcp.CallToolRequest, input searchInput) (*mcp.CallToolResult, any, error) {
		rows, code := provider.Search(ctx, input.Query)
		data, _ := json.Marshal(map[string]any{"query": input.Query, "provider": "bing-rss", "searched_at": time.Now().UTC().Format(time.RFC3339), "results": rows, "trust": "untrusted public search results"})
		return researchResult(request, log, path, "web_search", "public-search", string(data), code)
	})
}

func utf8Prefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}
