// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// This is a disposable integration-test worker, not a production agent.
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const socketPath = "/tmp/agenova-workerctl.sock"

type request struct {
	Action string `json:"action"`
	Claim  string `json:"claim"`
}

type response struct {
	State  string `json:"state,omitempty"`
	Claim  string `json:"claim,omitempty"`
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

type worker struct {
	mu      sync.Mutex
	claim   string
	state   string
	result  string
	command *exec.Cmd
	done    chan struct{}
}

func main() {
	if len(os.Args) < 2 {
		fail("usage: agenova-workerctl serve|start|stop|status")
	}
	switch os.Args[1] {
	case "serve":
		if len(os.Args) != 2 {
			fail("serve takes no arguments")
		}
		if err := serve(socketPath); err != nil {
			fail(err.Error())
		}
	case "_task":
		if len(os.Args) != 3 || !validClaim(os.Args[2]) {
			fail("invalid test task claim")
		}
		// A deterministic unit of real child-process work, then remain alive so
		// Terminate has a process to stop independently of Pod deletion.
		fmt.Printf("probe-ready claim=%s result=%s\n", os.Args[2], probeResult(os.Args[2]))
		for {
			time.Sleep(time.Hour)
		}
	case "start", "stop", "status":
		if len(os.Args) != 3 || !validClaim(os.Args[2]) {
			fail("one valid claim ID is required")
		}
		if err := client(socketPath, request{Action: os.Args[1], Claim: os.Args[2]}); err != nil {
			fail(err.Error())
		}
	default:
		fail("unknown command")
	}
}

func validClaim(claim string) bool {
	if claim == "" || len(claim) > 128 {
		return false
	}
	for _, c := range claim {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == ':') {
			return false
		}
	}
	return true
}

func probeResult(claim string) string {
	sum := sha256.Sum256([]byte("agenova-test-worker:" + claim))
	return "probe-" + hex.EncodeToString(sum[:8])
}

func serve(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	// A previous server must not be replaced. This also avoids unlinking a
	// live socket when a duplicate serve command is run via kubectl exec.
	if _, err := os.Lstat(path); err == nil {
		conn, dialErr := net.DialTimeout("unix", path, time.Second)
		if dialErr == nil {
			conn.Close()
			return errors.New("worker control server is already running")
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	defer listener.Close()
	defer os.Remove(path)
	if err := os.Chmod(path, 0600); err != nil {
		return err
	}
	w := &worker{state: "idle"}
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt)
	defer signal.Stop(shutdown)
	go func() {
		<-shutdown
		listener.Close()
	}()
	for {
		conn, err := listener.Accept()
		if err != nil {
			return nil // normally container shutdown
		}
		go func() {
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
			var req request
			if err := json.NewDecoder(io.LimitReader(conn, 1024)).Decode(&req); err != nil {
				_ = json.NewEncoder(conn).Encode(response{Error: "invalid request"})
				return
			}
			_ = json.NewEncoder(conn).Encode(w.handle(req))
		}()
	}
}

func (w *worker) handle(req request) response {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !validClaim(req.Claim) {
		return response{Error: "invalid claim ID"}
	}
	w.refresh()
	if w.claim != "" && w.claim != req.Claim {
		return response{Error: "claim does not own this worker"}
	}
	switch req.Action {
	case "start":
		if w.state != "idle" {
			return response{Error: "worker has already been started"}
		}
		cmd := exec.Command(os.Args[0], "_task", req.Claim)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return response{Error: "cannot create child output pipe"}
		}
		if err := cmd.Start(); err != nil {
			return response{Error: "cannot start child process"}
		}
		line := make(chan string, 1)
		go func() {
			value, _ := bufio.NewReader(stdout).ReadString('\n')
			line <- strings.TrimSpace(value)
		}()
		var ack string
		select {
		case ack = <-line:
		case <-time.After(3 * time.Second):
		}
		want := fmt.Sprintf("probe-ready claim=%s result=%s", req.Claim, probeResult(req.Claim))
		if ack != want {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return response{Error: "child did not acknowledge its claim and probe result"}
		}
		w.claim, w.state, w.result, w.command = req.Claim, "running", probeResult(req.Claim), cmd
		w.done = make(chan struct{})
		go func() {
			_ = cmd.Wait()
			close(w.done)
		}()
		return response{State: "started", Claim: w.claim, Result: w.result}
	case "stop":
		if w.state == "idle" {
			// A cancellation that wins the lock before Start permanently owns
			// this one-claim worker. A later Start cannot race past it.
			w.claim, w.state, w.result = req.Claim, "stopped", "none"
			return response{State: "stopped", Claim: w.claim, Result: w.result}
		}
		if w.state == "running" {
			if err := w.command.Process.Kill(); err != nil {
				select {
				case <-w.done: // It exited before the signal; still a confirmed stop.
				default:
					return response{Error: "cannot stop child process"}
				}
			}
			select {
			case <-w.done:
			case <-time.After(3 * time.Second):
				return response{Error: "child exit not confirmed"}
			}
			w.state = "stopped"
		}
		return response{State: "stopped", Claim: w.claim, Result: w.result}
	case "status":
		if w.state == "idle" {
			// A query does not reserve the worker for the queried claim.
			return response{State: "idle", Claim: req.Claim, Result: "none"}
		}
		return response{State: w.state, Claim: w.claim, Result: w.result}
	default:
		return response{Error: "unknown action"}
	}
}

func (w *worker) refresh() {
	if w.state != "running" {
		return
	}
	select {
	case <-w.done:
		w.state = "stopped"
	default:
	}
}

func client(path string, req request) error {
	conn, err := net.DialTimeout("unix", path, 3*time.Second)
	if err != nil {
		return fmt.Errorf("worker control server is not available: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return err
	}
	var res response
	if err := json.NewDecoder(io.LimitReader(conn, 1024)).Decode(&res); err != nil {
		return err
	}
	if res.Error != "" {
		return errors.New(res.Error)
	}
	if res.Claim != req.Claim || res.State == "" {
		return errors.New("worker returned an invalid claim-bound acknowledgement")
	}
	fmt.Printf("state=%s claim=%s result=%s\n", res.State, res.Claim, res.Result)
	return nil
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
