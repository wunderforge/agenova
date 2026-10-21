// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Command netprobe is a disposable acceptance observer, not an agent permission
// mechanism. Operator-pinned argv selects targets; no Work/task/token is read.
package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

const mcpCanary = "http://e16-mcp.agenova-e16.svc.cluster.local:8080/healthz"
const maxReportBytes = 2 << 20
const maxRecords = 8192

type config struct {
	mode, host, ip, dns, campaign, mcpIP string
	duration, interval, timeout          time.Duration
	hold                                 bool
	// Test-only injected coordinates/CA; main never accepts them from argv.
	publicAddress, canary, mcpAddress string
	tlsConfig                         *tls.Config
	resolver                          *net.Resolver
}

type record struct {
	Time       string `json:"time"`
	Nonce      string `json:"nonce"`
	Sequence   int    `json:"sequence"`
	Event      string `json:"event"`
	Target     string `json:"target,omitempty"`
	TargetHash string `json:"targetHash,omitempty"`
	Outcome    string `json:"outcome,omitempty"`
	Millis     int64  `json:"millis,omitempty"`
	HTTPStatus int    `json:"httpStatus,omitempty"`
	Connection int    `json:"connection,omitempty"`
	Attempt    int    `json:"attemptSequence,omitempty"`
}

type reporter struct {
	mu         sync.Mutex
	out        io.Writer
	nonce      string
	seq, bytes int
	stopped    bool
}

func (r *reporter) emit(v record) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return false
	}
	if r.seq >= maxRecords-1 || r.bytes > maxReportBytes-1024 {
		v = record{Event: "native-probe-end", Outcome: "report-limit; incomplete-observation"}
		r.stopped = true
	}
	r.seq++
	v.Sequence, v.Nonce, v.Time = r.seq, r.nonce, time.Now().UTC().Format(time.RFC3339Nano)
	if v.Event == "native-network-attempt" {
		v.Attempt = r.seq
	}
	data, _ := json.Marshal(v)
	n, err := r.out.Write(append(data, '\n'))
	r.bytes += n
	if err != nil {
		r.stopped = true
	}
	return !r.stopped
}

func (r *reporter) currentSequence() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.seq
}

func main() {
	c := config{canary: mcpCanary}
	flag.StringVar(&c.mode, "mode", "once", "once (stdout receipts) or wrap (stderr receipts plus unchanged worker serve)")
	flag.StringVar(&c.host, "public-host", "", "operator-approved TLS hostname; no URL, path or credentials")
	flag.StringVar(&c.ip, "public-ip", "", "operator-approved public IPv4 pinned to TCP443; verify hostname mapping independently")
	flag.StringVar(&c.dns, "dns-name", "", "operator-approved DNS name, default public-host")
	flag.StringVar(&c.mcpIP, "mcp-ip", "", "optional reviewed private IPv4 of e16-mcp Service; bypasses DNS, fixed8080/healthz only")
	flag.StringVar(&c.campaign, "nonce", "", "optional public campaign label, 8..64 ASCII letters/digits/hyphens")
	flag.DurationVar(&c.duration, "duration", 10*time.Minute, "probe duration, at most30m; does not alter Work expiry")
	flag.DurationVar(&c.interval, "interval", 2*time.Second, "probe interval, 500ms..30s")
	flag.DurationVar(&c.timeout, "timeout", 500*time.Millisecond, "per-target timeout, 100ms..5s")
	flag.BoolVar(&c.hold, "hold", false, "also sample one established verified TLS/HEAD connection; peer closure is inconclusive")
	flag.Parse()
	if err := validate(c); err != nil {
		fmt.Fprintln(os.Stderr, "netprobe: invalid operator configuration; no probes started")
		os.Exit(2)
	}
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		fmt.Fprintln(os.Stderr, "netprobe: cannot create observation nonce")
		os.Exit(2)
	}
	nonce := hex.EncodeToString(random[:])
	if c.campaign != "" {
		nonce = c.campaign + "-" + nonce
	}
	c.publicAddress = net.JoinHostPort(c.ip, "443")
	if c.mcpIP != "" {
		c.mcpAddress = net.JoinHostPort(c.mcpIP, "8080")
	}
	c.resolver = &net.Resolver{PreferGo: true}
	out := io.Writer(os.Stdout)
	if c.mode == "wrap" {
		out = os.Stderr // never contaminate worker/control protocol stdout
	}
	r := &reporter{out: out, nonce: nonce}
	if c.mode == "once" {
		ctx, cancel := context.WithTimeout(context.Background(), 4*c.timeout)
		defer cancel()
		connection := &held{}
		defer func() {
			if connection.conn != nil {
				connection.conn.Close()
			}
		}()
		probeRound(ctx, c, r, connection)
		return // outcome receipts, not exit0, decide connectivity
	}
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	os.Exit(wrap(context.Background(), c, r, []string{"/agenova-workerctl", "serve"}, signals, os.Stdin, os.Stdout, os.Stderr))
}

func validate(c config) error {
	if (c.mode != "once" && c.mode != "wrap") || !hostname(c.host) || (c.dns != "" && !hostname(c.dns)) || c.duration <= 0 || c.duration > 30*time.Minute || c.interval < 500*time.Millisecond || c.interval > 30*time.Second || c.timeout < 100*time.Millisecond || c.timeout > 5*time.Second {
		return errors.New("invalid target or bounds")
	}
	ip, err := netip.ParseAddr(c.ip)
	if err != nil || !publicIPv4(ip) {
		return errors.New("target is not public IPv4")
	}
	if c.mcpIP != "" {
		mcp, err := netip.ParseAddr(c.mcpIP)
		if err != nil || !mcp.Is4() || !mcp.IsPrivate() {
			return errors.New("MCP pin must be private IPv4")
		}
	}
	if c.campaign != "" {
		if len(c.campaign) < 8 || len(c.campaign) > 64 {
			return errors.New("invalid campaign")
		}
		for _, ch := range c.campaign {
			if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-') {
				return errors.New("invalid campaign")
			}
		}
	}
	return nil
}

func hostname(s string) bool {
	if len(s) > 253 || !strings.Contains(s, ".") || strings.TrimSpace(s) != s {
		return false
	}
	if _, err := netip.ParseAddr(s); err == nil {
		return false
	}
	for _, label := range strings.Split(s, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, ch := range label {
			if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-') {
				return false
			}
		}
	}
	return true
}

func publicIPv4(ip netip.Addr) bool {
	if !ip.Is4() || !ip.IsGlobalUnicast() {
		return false
	}
	for _, value := range []string{"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.168.0.0/16", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4"} {
		if netip.MustParsePrefix(value).Contains(ip) {
			return false
		}
	}
	return true
}

func result(err error) string {
	if err == nil {
		return "reachable"
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled; observation-incomplete"
	}
	var verification *tls.CertificateVerificationError
	if errors.As(err, &verification) {
		return "TLS-verification-failed; invalid-positive-control"
	}
	var dns *net.DNSError
	if errors.As(err, &dns) && dns.IsNotFound {
		return "DNS-negative; invalid-positive-control"
	}
	var timeout net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &timeout) && timeout.Timeout() {
		return "timeout; enforcement-unverified"
	}
	return "connection-error; enforcement-unverified"
}

func targetHash(target string) string {
	sum := sha256.Sum256([]byte(target))
	return hex.EncodeToString(sum[:8])
}

type held struct {
	conn   net.Conn
	serial int
}

func probeRound(ctx context.Context, c config, r *reporter, connection *held) {
	// Operator pins IP and TLS hostname separately so this first attempt does
	// not depend on DNS. No proxy, redirects, task input or credentials.
	probe := func(target, binding string, do func(context.Context) record) bool {
		hash := targetHash(binding)
		if !r.emit(record{Event: "native-network-attempt", Target: target, TargetHash: hash}) {
			return false
		}
		attempt := r.currentSequence()
		call, cancel := context.WithTimeout(ctx, c.timeout)
		start := time.Now()
		v := do(call)
		cancel()
		v.Event, v.Target, v.TargetHash, v.Millis, v.Attempt = "native-network-result", target, hash, time.Since(start).Milliseconds(), attempt
		return r.emit(v)
	}
	if !probe("public-tls", c.publicAddress+"/"+c.host, func(call context.Context) record {
		conf := &tls.Config{ServerName: c.host, MinVersion: tls.VersionTLS12}
		if c.tlsConfig != nil {
			conf = c.tlsConfig.Clone()
			conf.ServerName = c.host
		}
		dialer := tls.Dialer{NetDialer: &net.Dialer{}, Config: conf}
		conn, err := dialer.DialContext(call, "tcp4", c.publicAddress)
		if err != nil {
			return record{Outcome: result(err)}
		}
		if connection != nil && c.hold && connection.conn == nil {
			connection.conn, connection.serial = conn, connection.serial+1
		} else {
			conn.Close()
		}
		return record{Outcome: "verified-TLS-reachable"}
	}) {
		return
	}
	dnsName := c.dns
	if dnsName == "" {
		dnsName = c.host
	}
	if !probe("dns", dnsName, func(call context.Context) record {
		resolver := c.resolver
		if resolver == nil {
			resolver = &net.Resolver{PreferGo: true}
		}
		addresses, err := resolver.LookupHost(call, dnsName)
		if err == nil && len(addresses) == 0 {
			return record{Outcome: "empty-DNS; invalid-positive-control"}
		}
		return record{Outcome: result(err)}
	}) {
		return
	}
	if !probe("direct-mcp-canary", c.canary+"@"+c.mcpAddress, func(call context.Context) record {
		transport := &http.Transport{Proxy: nil, DisableKeepAlives: true}
		if c.mcpAddress != "" {
			transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "tcp4", c.mcpAddress)
			}
		}
		defer transport.CloseIdleConnections()
		client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		request, _ := http.NewRequestWithContext(call, http.MethodGet, c.canary, nil)
		response, err := client.Do(request)
		if err != nil {
			return record{Outcome: result(err)}
		}
		response.Body.Close() // never collect or log response content
		return record{Outcome: "HTTP-reachable", HTTPStatus: response.StatusCode}
	}) {
		return
	}
	if connection != nil && connection.conn != nil {
		probe("held-tls", c.publicAddress+"/"+c.host, func(context.Context) record {
			connection.conn.SetDeadline(time.Now().Add(c.timeout))
			req := &http.Request{Method: http.MethodHead, URL: nil, Host: c.host, Header: make(http.Header)}
			// Public, generated correlation only: independent proxy logs can
			// confirm this attempt without receiving credentials or task input.
			_, err := fmt.Fprintf(connection.conn, "HEAD /?agenova_network_probe=%s&probe_attempt=%d&connection=%d HTTP/1.1\r\nHost: %s\r\nUser-Agent: agenova-native-network-probe\r\nConnection: keep-alive\r\n\r\n", r.nonce, r.currentSequence(), connection.serial, c.host)
			if err == nil {
				var response *http.Response
				response, err = http.ReadResponse(bufio.NewReader(io.LimitReader(connection.conn, 8192)), req)
				if err == nil {
					response.Body.Close()
					if !response.Close {
						return record{Outcome: "existing-connection-response", HTTPStatus: response.StatusCode, Connection: connection.serial}
					}
				}
			}
			connection.conn.Close()
			connection.conn = nil
			return record{Outcome: "connection-closed-or-timeout; cause-unverified", Connection: connection.serial}
		})
	}
}

func loop(ctx context.Context, c config, r *reporter, started chan<- struct{}) {
	defer r.emit(record{Event: "native-probe-end", Outcome: "observer-ended; not-proof-of-worker-termination"})
	// Commit startup observation before launching the unchanged control server.
	r.emit(record{Event: "native-probe-start", Outcome: "observer-only; independent-positive-controls-required"})
	close(started)
	connection := &held{}
	defer func() {
		if connection.conn != nil {
			connection.conn.Close()
		}
	}()
	for {
		if ctx.Err() != nil {
			return
		}
		probeRound(ctx, c, r, connection)
		r.mu.Lock()
		stopped := r.stopped
		r.mu.Unlock()
		if stopped {
			return
		}
		timer := time.NewTimer(c.interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func wrap(ctx context.Context, c config, r *reporter, worker []string, signals <-chan os.Signal, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(worker) == 0 {
		return 2
	}
	probeCtx, cancel := context.WithTimeout(ctx, c.duration)
	defer cancel()
	started, probeDone := make(chan struct{}), make(chan struct{})
	go func() { defer close(probeDone); loop(probeCtx, c, r, started) }()
	<-started
	cmd := exec.Command(worker[0], worker[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	if err := cmd.Start(); err != nil {
		cancel()
		<-probeDone
		r.emit(record{Event: "worker-exit", Outcome: "start-failed"})
		return 2
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var exitErr error
	select {
	case exitErr = <-done:
	case sig := <-signals:
		_ = cmd.Process.Signal(sig)
		select {
		case exitErr = <-done:
		case <-time.After(3 * time.Second):
			_ = cmd.Process.Kill()
			exitErr = <-done
		}
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		exitErr = <-done
	}
	cancel()
	<-probeDone
	if exitErr == nil {
		return 0
	}
	var status *exec.ExitError
	if errors.As(exitErr, &status) && status.ExitCode() >= 0 {
		return status.ExitCode()
	}
	if errors.As(exitErr, &status) {
		if wait, ok := status.Sys().(syscall.WaitStatus); ok && wait.Signaled() {
			return 128 + int(wait.Signal())
		}
	}
	return 1
}
