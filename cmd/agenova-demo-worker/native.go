// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"github.com/wunderforge/agenova/internal/workerprotocol"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"time"
)

// nativeFallbackIP may be pinned at image build after verifying public DNS.
// It pins the one fixed host for same-IP allow/deny controls. TLS still validates
// the hostname; this is not an alternate URL or a networking grant.
var nativeFallbackIP string

func publicIP(ip net.IP) bool {
	return ip != nil && ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsUnspecified()
}

func nativeHTTPS(target string) (workerprotocol.NativeObservation, string) {
	result := workerprotocol.NativeObservation{Phase: "dns", Result: "error"}
	if target != workerprotocol.NativeTarget {
		return result, "Unapproved diagnostic target."
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var chosen net.IP
	var err error
	if publicIP(net.ParseIP(nativeFallbackIP)) {
		chosen = net.ParseIP(nativeFallbackIP)
	} else {
		dnsCtx, dnsCancel := context.WithTimeout(ctx, 2*time.Second)
		var ips []net.IP
		ips, err = net.DefaultResolver.LookupIP(dnsCtx, "ip4", "jitsi.org")
		dnsCancel()
		if err == nil {
			for _, ip := range ips {
				if publicIP(ip) {
					chosen = ip
					break
				}
			}
		}
	}
	if chosen == nil {
		if errors.Is(err, context.DeadlineExceeded) {
			result.Result = "timeout"
		}
		result.Milliseconds = time.Since(start).Milliseconds()
		return result, "No public address resolved; this alone does not prove HTTPS blocking."
	}
	result.IP = chosen.String()
	result.Phase = "tcp"
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, "tcp", net.JoinHostPort(chosen.String(), "443"))
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	trace := &httptrace.ClientTrace{TLSHandshakeStart: func() { result.Phase = "tls" }, TLSHandshakeDone: func(_ tls.ConnectionState, e error) {
		if e == nil {
			result.Phase = "http"
		}
	}}
	req, _ := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), http.MethodGet, target, nil)
	req.Header.Set("User-Agent", "Agenova-Demo-Native-Probe/1.0")
	response, err := client.Do(req)
	if err != nil {
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			result.Result = "timeout"
		}
		result.Milliseconds = time.Since(start).Milliseconds()
		return result, "Direct HTTPS did not return page content. This is a native-client observation, not a verified infrastructure verdict."
	}
	defer response.Body.Close()
	result.StatusCode = response.StatusCode
	result.Phase = "http"
	data, readErr := io.ReadAll(io.LimitReader(response.Body, 16384))
	result.Milliseconds = time.Since(start).Milliseconds()
	if readErr != nil {
		var ne net.Error
		if errors.As(readErr, &ne) && ne.Timeout() {
			result.Result = "timeout"
		}
		return result, "Response body could not be read completely."
	}
	result.Result = "received"
	return result, "[UNTRUSTED PUBLIC PAGE DATA] Source: " + target + "\n" + string(data)
}
