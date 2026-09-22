// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// role-demo-submit submits one real ClaimRequest to an explicitly selected
// loopback console and prints the final evidence without inventing results.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/evidence"
)

func main() {
	endpoint := flag.String("endpoint", "", "explicit loopback console URL")
	file := flag.String("file", "", "ClaimRequest YAML")
	fullJSON := flag.Bool("json", false, "print full final evidence JSON")
	flag.Parse()
	u, err := url.Parse(*endpoint)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		fail("invalid endpoint")
	}
	host, _, err := net.SplitHostPort(u.Host)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		fail("endpoint must be literal loopback")
	}
	data, err := os.ReadFile(*file)
	if err != nil {
		fail("request file unavailable")
	}
	request, validationErr := v0.ParseClaimRequestYAML(data)
	if validationErr != nil {
		fail("invalid ClaimRequest: " + validationErr.Error())
	}
	encoded, _ := json.Marshal(request)
	client := &http.Client{Timeout: 20 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 9*time.Minute)
	defer cancel()
	post, err := http.NewRequestWithContext(ctx, http.MethodPost, *endpoint+"/api/requests", bytes.NewReader(encoded))
	if err != nil {
		fail("cannot build request")
	}
	post.Header.Set("Content-Type", "application/json")
	response, err := client.Do(post)
	if err != nil {
		fail("submission transport failed: " + err.Error())
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	response.Body.Close()
	if err != nil || response.StatusCode != http.StatusAccepted {
		fail(fmt.Sprintf("submission HTTP %d: %s", response.StatusCode, body))
	}
	fmt.Printf("Submitted %s to %s (HTTP %d). Waiting for final evidence...\n", request.Metadata.Name, *endpoint, response.StatusCode)
	for ctx.Err() == nil {
		time.Sleep(2 * time.Second)
		poll, err := http.NewRequestWithContext(ctx, http.MethodGet, *endpoint+"/api/requests/"+url.PathEscape(request.Metadata.Name)+"/evidence", nil)
		if err != nil {
			fail("cannot build evidence request")
		}
		result, err := client.Do(poll)
		if err != nil {
			fail("evidence transport failed: " + err.Error())
		}
		body, err = io.ReadAll(io.LimitReader(result.Body, 1<<20))
		result.Body.Close()
		if err != nil || result.StatusCode != http.StatusOK {
			fail(fmt.Sprintf("evidence HTTP %d", result.StatusCode))
		}
		var view evidence.View
		if json.Unmarshal(body, &view) != nil {
			fail("invalid evidence JSON")
		}
		if view.Outcome != nil && (view.Outcome.Status == "Succeeded" || view.Outcome.Status == "Failed" || view.Outcome.Status == "Expired" || view.Outcome.Status == "Cancelled") {
			fmt.Printf("Final %s: %s\n", request.Metadata.Name, view.Outcome.Status)
			if view.State != nil {
				fmt.Printf("Trusted team: %s\n", view.State.Principal.Team)
				if view.State.EffectiveAuthority != nil {
					fmt.Printf("Effective tools: %v\n", view.State.EffectiveAuthority.Tools)
				}
			}
			for _, fact := range view.Facts {
				if fact.Kind == "ToolDecision" && fact.Target != "git.read" {
					fmt.Printf("Tool Gateway: %s %s (%s)\n", fact.Target, fact.Result, fact.ReasonCode)
				}
				if fact.Kind == "ProviderOutcome" && fact.Operation == "tool.invoke" {
					fmt.Printf("Provider: %s %s (%s)\n", fact.Target, fact.ProviderStatus, fact.ReasonCode)
				}
			}
			if view.Outcome.Failure != "" {
				fmt.Printf("Failure: %s\n", view.Outcome.Failure)
			}
			if *fullJSON {
				fmt.Printf("Evidence JSON: %s\n", body)
			}
			return
		}
	}
	fail("timed out waiting for Work")
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
