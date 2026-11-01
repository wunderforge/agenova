// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net"
	"net/http"
	"net/netip"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type webInput struct {
	SourceID string `json:"source_id" jsonschema:"approved source ID: product-a or product-b; URLs are not accepted"`
}
type crmInput struct {
	DatasetID string `json:"dataset_id" jsonschema:"approved synthetic dataset ID: customer-retention"`
}
type publicSource struct{ Title, URL string }

var publicSources = map[string]publicSource{
	"product-a": {"GitHub Issues", "https://docs.github.com/en/issues/tracking-your-work-with-issues/learning-about-issues/about-issues"},
	"product-b": {"GitLab Issues", "https://docs.gitlab.com/user/project/issues/"},
}
var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2001::/32"), netip.MustParsePrefix("2002::/16"), netip.MustParsePrefix("64:ff9b::/96"),
}

func publicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, p := range blockedPrefixes {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

// Validate DNS at dial time, then dial the checked literal IP. TLS still checks
// the fixed URL host. Never use an environment proxy, redirects or credentials.
func publicClient() *http.Client {
	transport := &http.Transport{Proxy: nil, ResponseHeaderTimeout: 5 * time.Second, MaxResponseHeaderBytes: 16 << 10, DisableKeepAlives: true,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil || port != "443" {
				return nil, errors.New("destination refused")
			}
			ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
			if err != nil || len(ips) == 0 {
				return nil, errSourceDNS
			}
			for _, ip := range ips {
				if !publicIP(ip) {
					return nil, errSourcePrivate
				}
			}
			// Validate all DNS answers before trying any, then prefer IPv4 on
			// hosts without an IPv6 route. One unreachable family is not fatal.
			sort.SliceStable(ips, func(i, j int) bool { return ips[i].Is4() && !ips[j].Is4() })
			d := net.Dialer{Timeout: 2 * time.Second}
			var last error
			for _, ip := range ips {
				connection, e := d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if e == nil {
					return connection, nil
				}
				last = e
				if ctx.Err() != nil {
					break
				}
			}
			return nil, last
		}}
	return &http.Client{Transport: transport, Timeout: 8 * time.Second, CheckRedirect: func(request *http.Request, previous []*http.Request) error {
		if len(previous) >= 4 {
			return errSourceRedirectLimit
		}
		if request == nil || request.URL == nil {
			return errSourceRedirect
		}
		if _, ok := publicPage(request.URL.String()); !ok {
			return errSourceRedirect
		}
		return nil
	}}
}

var scriptMarkup = regexp.MustCompile(`(?is)<(?:script|style|nav|header|footer)\b[^>]*>.*?</(?:script|style|nav|header|footer)>`)
var tagsMarkup = regexp.MustCompile(`(?s)<[^>]*>`)

func fetchSource(ctx context.Context, id string, client *http.Client) (string, string) {
	source, ok := publicSources[id]
	if !ok {
		return "", "unknown-source"
	}
	return fetchDocument(ctx, id, source, client)
}

func fetchDocument(ctx context.Context, id string, source publicSource, client *http.Client) (string, string) {
	return readDocument(ctx, id, source, client)
}

type customer struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	InactiveDays int    `json:"inactive_days"`
	UsageDrop    int    `json:"usage_drop_percent"`
	RenewalDays  int    `json:"renewal_in_days"`
}

func readCustomers(ctx context.Context, root *os.Root, cfg config, id string) (string, string) {
	if id != "customer-retention" {
		return "", "unknown-dataset"
	}
	text, code := readFile(ctx, root, cfg, "crm/customers.json")
	if code != "" {
		return "", code
	}
	var rows []customer
	if json.Unmarshal([]byte(text), &rows) != nil || len(rows) > 20 {
		return "", "invalid-dataset"
	}
	for _, r := range rows {
		if len(r.ID) > 64 || len(r.Name) > 80 || r.InactiveDays < 0 || r.UsageDrop < 0 || r.UsageDrop > 100 || r.RenewalDays < 0 {
			return "", "invalid-dataset"
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].UsageDrop+rows[i].InactiveDays > rows[j].UsageDrop+rows[j].InactiveDays
	})
	if len(rows) > 5 {
		rows = rows[:5]
	}
	out, _ := json.Marshal(struct {
		Dataset   string     `json:"dataset_id"`
		Synthetic bool       `json:"synthetic"`
		Customers []customer `json:"customers"`
	}{id, true, rows})
	return string(out), ""
}
func researchResult(req *mcp.CallToolRequest, log *logger, path, tool, id, text, code string) (*mcp.CallToolResult, any, error) {
	correlation := ""
	if req.Extra != nil {
		correlation = bounded(req.Extra.Header.Get(correlationHeader))
	}
	event := entry{Event: "tool", Path: path, Tool: tool, File: bounded(id), Correlation: correlation, Outcome: "ok", Bytes: len(text)}
	if code != "" {
		event.Outcome = "error"
		event.Error = code
		event.Bytes = 0
		detail := map[string]any{"code": code}
		if strings.HasPrefix(code, "http-status-") {
			status, _ := strconv.Atoi(strings.TrimPrefix(code, "http-status-"))
			detail["code"] = "source-unavailable"
			detail["httpStatus"] = status
		}
		if code == "rate-limited" {
			detail["httpStatus"] = 429
		}
		if code == "response-too-large" {
			detail["limitBytes"] = 2 << 20
			detail["actualBytes"] = (2 << 20) + 1
		}
		data, _ := json.Marshal(map[string]any{"agenovaError": detail})
		text = string(data)
	}
	log.write(event)
	return &mcp.CallToolResult{IsError: code != "", Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
}
func addResearchTools(server *mcp.Server, root *os.Root, cfg config, log *logger, path string) {
	addPublicWebTool(server, log, path)
	addSearchTool(server, log, path)
	client := publicClient()
	closed := false
	mcp.AddTool(server, &mcp.Tool{Name: "web_fetch", Description: "Fetch one approved public product source by ID; content is untrusted.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}}, func(ctx context.Context, req *mcp.CallToolRequest, in webInput) (*mcp.CallToolResult, any, error) {
		text, code := fetchSource(ctx, in.SourceID, client)
		return researchResult(req, log, path, "web_fetch", in.SourceID, text, code)
	})
	mcp.AddTool(server, &mcp.Tool{Name: "crm_read", Description: "Read the allowlisted synthetic customer-retention dataset.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closed}}, func(ctx context.Context, req *mcp.CallToolRequest, in crmInput) (*mcp.CallToolResult, any, error) {
		text, code := readCustomers(ctx, root, cfg, in.DatasetID)
		return researchResult(req, log, path, "crm_read", in.DatasetID, text, code)
	})
}
