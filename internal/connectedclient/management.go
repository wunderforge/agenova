// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package connectedclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/wunderforge/agenova/internal/registration"
	"io"
	"net/http"
	"strings"
	"time"
)

func (c Client) management(path string, input any, out any) error {
	if !validServerOrigin(c.Server) || c.AccessToken == nil {
		return errors.New("authenticated HTTPS connection required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	b, err := json.Marshal(input)
	if err != nil || len(b) > (1<<20)+4096 {
		return errors.New("management request too large")
	}
	token, err := c.AccessToken(ctx)
	if err != nil || token == "" || len(token) > 16<<10 || strings.ContainsAny(token, " \t\r\n") {
		return errors.New("sign in required")
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.Server+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	defer req.Header.Del("Authorization")
	req.Header.Set("Content-Type", "application/json")
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Transport: &http.Transport{Proxy: nil}}
	}
	safe := *client
	safe.Timeout = 30 * time.Second
	safe.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := safe.Do(req)
	if err != nil {
		return errors.New("management service unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("management request rejected (HTTP %d)", response.StatusCode)
	}
	if out == nil {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(data) > 1<<20 || !decodeStrictJSON(data, out) {
		return errors.New("invalid management response")
	}
	return nil
}
func (c Client) ApplyGovernance(project, kind, document string) (registration.Result, error) {
	var result registration.Result
	err := c.management("/api/governance", map[string]string{"project": project, "kind": kind, "document": document}, &result)
	return result, err
}
func (c Client) AuthorizePlatform(installation, action string) error {
	return c.management("/api/platform/authorize", map[string]string{"installation": installation, "action": action}, nil)
}
