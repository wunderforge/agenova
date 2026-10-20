// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func valid() config {
	return config{mode: "once", host: "example.com", ip: "1.1.1.1", canary: mcpCanary, duration: time.Minute, interval: time.Second, timeout: time.Second}
}

func TestOperatorTargetsAndBoundsFailClosed(t *testing.T) {
	if validate(valid()) != nil {
		t.Fatal("valid operator configuration rejected")
	}
	for _, value := range []string{"", "127.0.0.1", "169.254.169.254", "10.1.2.3", "100.64.0.1", "192.0.2.1", "::1", "8.8.8.8:443"} {
		c := valid()
		c.ip = value
		if validate(c) == nil {
			t.Fatalf("non-public target accepted: %q", value)
		}
	}
	for _, value := range []string{"https://example.com", "user:password@example.com", "example.com/path", "127.0.0.1", "-bad.example", "x..example", "example.com\n"} {
		c := valid()
		c.host = value
		if validate(c) == nil {
			t.Fatal("invalid TLS hostname accepted")
		}
	}
	c := valid()
	c.duration = time.Hour
	if validate(c) == nil {
		t.Fatal("unbounded probe duration")
	}
	c = valid()
	c.campaign = "not-a-token\n"
	if validate(c) == nil {
		t.Fatal("unsafe campaign label")
	}
	for _, ip := range []string{"127.0.0.1", "169.254.169.254", "1.1.1.1", "::1"} {
		c = valid()
		c.mcpIP = ip
		if validate(c) == nil {
			t.Fatal("nonprivate or ambiguous MCP pin accepted")
		}
	}
	c = valid()
	c.mcpIP = "172.20.10.7"
	if validate(c) != nil {
		t.Fatal("private MCP Service pin refused")
	}
}

// Isolated UDP DNS positive control: no external DNS or resolver environment.
func localDNS(t *testing.T) *net.Resolver {
	t.Helper()
	listener, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		buffer := make([]byte, 2048)
		for {
			n, peer, err := listener.ReadFrom(buffer)
			if err != nil {
				return
			}
			request := append([]byte(nil), buffer[:n]...)
			if len(request) < 17 {
				continue
			}
			end := 12
			for end < len(request) && request[end] != 0 {
				end += int(request[end]) + 1
			}
			end++
			if end+4 > len(request) {
				continue
			}
			kind := binary.BigEndian.Uint16(request[end : end+2])
			response := append([]byte(nil), request[:end+4]...)
			binary.BigEndian.PutUint16(response[2:4], 0x8180)
			binary.BigEndian.PutUint16(response[4:6], 1)
			binary.BigEndian.PutUint16(response[6:8], 0)
			binary.BigEndian.PutUint16(response[8:10], 0)
			binary.BigEndian.PutUint16(response[10:12], 0)
			if kind == 1 {
				binary.BigEndian.PutUint16(response[6:8], 1)
				response = append(response, 0xc0, 0x0c, 0, 1, 0, 1, 0, 0, 0, 1, 0, 4, 127, 0, 0, 1)
			}
			listener.WriteTo(response, peer)
		}
	}()
	return &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "udp4", listener.LocalAddr().String())
	}}
}

func records(t *testing.T, output []byte) []record {
	t.Helper()
	var values []record
	for _, line := range bytes.Split(output, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var value record
		if json.Unmarshal(line, &value) != nil {
			t.Fatal("invalid observation NDJSON")
		}
		values = append(values, value)
	}
	return values
}

func TestLocalPositiveControlsAndHeldConnectionUseVerifiedTLS(t *testing.T) {
	var heads, canaries atomic.Int32
	public := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" || r.URL.Path != "/" || r.Header.Get("Authorization") != "" || r.URL.Query().Get("agenova_network_probe") != "synthetic-local-campaign" || r.URL.Query().Get("probe_attempt") == "" {
			t.Error("probe sent unexpected request content")
		}
		heads.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer public.Close()
	public.Config.ErrorLog = log.New(io.Discard, "", 0)
	canary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" || r.Header.Get("Authorization") != "" {
			t.Error("canary accessed tool/data or credentials")
		}
		canaries.Add(1)
		fmt.Fprintln(w, "SYNTHETIC-NOT-LOGGED")
	}))
	defer canary.Close()
	root := x509.NewCertPool()
	root.AddCert(public.Certificate())
	c := valid()
	c.publicAddress, c.canary, c.resolver = strings.TrimPrefix(public.URL, "https://"), canary.URL+"/healthz", localDNS(t)
	c.tlsConfig, c.hold = &tls.Config{RootCAs: root, MinVersion: tls.VersionTLS12}, true
	var output bytes.Buffer
	r := &reporter{out: &output, nonce: "synthetic-local-campaign"}
	connection := &held{}
	defer func() {
		if connection.conn != nil {
			connection.conn.Close()
		}
	}()
	probeRound(context.Background(), c, r, connection)
	probeRound(context.Background(), c, r, connection)
	if heads.Load() != 2 || canaries.Load() != 2 || connection.serial != 1 {
		t.Fatalf("positive controls not effective or held connection replaced: heads=%d canary=%d connection=%d", heads.Load(), canaries.Load(), connection.serial)
	}
	results := map[string]int{}
	for _, entry := range records(t, output.Bytes()) {
		if entry.Event == "native-network-result" {
			results[entry.Target+"/"+entry.Outcome]++
		}
		if entry.Nonce == "" || entry.Time == "" || entry.Sequence == 0 || entry.TargetHash == "" {
			t.Fatal("observation missing bounded correlation")
		}
	}
	if results["public-tls/verified-TLS-reachable"] != 2 || results["dns/reachable"] != 2 || results["direct-mcp-canary/HTTP-reachable"] != 2 || results["held-tls/existing-connection-response"] != 2 {
		t.Fatalf("unexpected outcomes: %+v", results)
	}
	if strings.Contains(output.String(), "SYNTHETIC-NOT-LOGGED") || strings.Contains(output.String(), c.host) {
		t.Fatal("payload or raw target escaped bounded receipts")
	}
	// A genuine TLS validation failure is not evidence of a policy block.
	c.tlsConfig, c.hold = nil, false
	probeRound(context.Background(), c, r, nil)
	if !strings.Contains(output.String(), "TLS-verification-failed; invalid-positive-control") {
		t.Fatal("untrusted TLS server misclassified as denied")
	}
}

func TestFailureCategoriesDoNotInventEnforcement(t *testing.T) {
	if result(context.DeadlineExceeded) != "timeout; enforcement-unverified" || result(context.Canceled) != "cancelled; observation-incomplete" || result(errors.New("SYNTHETIC-RAW-ERROR")) != "connection-error; enforcement-unverified" {
		t.Fatal("negative outcomes overstate enforcement")
	}
	if result(&net.DNSError{IsNotFound: true}) != "DNS-negative; invalid-positive-control" {
		t.Fatal("NXDOMAIN reported as blocked")
	}
}

func TestPinnedMCPCanaryDoesNotDependOnDNS(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" || r.Host != "e16-mcp.agenova-e16.svc.cluster.local:8080" {
			t.Error("MCP Host/path changed")
		}
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	c := valid()
	c.timeout, c.publicAddress, c.resolver = 100*time.Millisecond, "127.0.0.1:1", localDNS(t)
	c.mcpAddress = server.Listener.Addr().String() // local-only injected port
	var output bytes.Buffer
	probeRound(context.Background(), c, &reporter{out: &output, nonce: "local-mcp-pin"}, nil)
	if calls.Load() != 1 || !strings.Contains(output.String(), "HTTP-reachable") {
		t.Fatal("pinned MCP canary tried to resolve cluster DNS or missed server")
	}
}

func TestReportIsBounded(t *testing.T) {
	var output bytes.Buffer
	r := &reporter{out: &output, nonce: "test-only"}
	for i := 0; i < maxRecords+10; i++ {
		r.emit(record{Event: "test", Target: "public-tls"})
	}
	if output.Len() > maxReportBytes || len(records(t, output.Bytes())) != maxRecords || !strings.Contains(output.String(), "report-limit; incomplete-observation") {
		t.Fatal("observation cap missing")
	}
}

func TestChildExitAndStdoutArePreserved(t *testing.T) {
	c := valid()
	c.duration, c.timeout = time.Second, 100*time.Millisecond
	c.publicAddress = "127.0.0.1:1"
	c.canary = "http://127.0.0.1:1/healthz"
	c.resolver = localDNS(t)
	var receipts, stdout, stderr bytes.Buffer
	r := &reporter{out: &receipts, nonce: "test-child"}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	code := wrap(context.Background(), c, r, []string{executable, "-test.run=^TestNetprobeChild$", "--", "netprobe-child-exit7"}, nil, nil, &stdout, &stderr)
	if code != 7 || stdout.String() != "worker-control-stdout\n" || strings.Contains(stdout.String(), "native-network") || !strings.Contains(receipts.String(), "native-probe-start") {
		t.Fatalf("child contract changed: exit=%d stdout=%q", code, stdout.String())
	}
}

func TestNetprobeChild(t *testing.T) {
	for _, value := range os.Args {
		if value == "netprobe-child-exit7" {
			fmt.Fprintln(os.Stdout, "worker-control-stdout")
			os.Exit(7)
		}
	}
}
