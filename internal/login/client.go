// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package login implements the native member OAuth flow. Credentials are kept
// in OS storage; installation state and Kubernetes never participate.
package login

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/99designs/keyring"
	"golang.org/x/oauth2"
)

const Callback = "http://127.0.0.1:38123/callback"

var ErrLogin = errors.New("sign-in failed; check the service configuration and active invitation")
var ErrCredentials = errors.New("OS credentials unavailable; sign in with agenova login")

type Config struct {
	SessionMode           string   `json:"sessionMode,omitempty"`
	Issuer                string   `json:"issuer"`
	ClientID              string   `json:"clientId"`
	AuthorizationEndpoint string   `json:"authorizationEndpoint"`
	TokenEndpoint         string   `json:"tokenEndpoint"`
	Scopes                []string `json:"scopes"`
	RedirectURI           string   `json:"redirectUri"`
}

func Origin(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Path == "" && u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" && !strings.Contains(raw, "#") && u.Opaque == ""
}
func (c Config) Validate() error {
	if c.SessionMode != "" && c.SessionMode != "application" {
		return ErrLogin
	}
	if c.SessionMode == "application" && (!Origin(c.Issuer) || c.ClientID != "agenova-cli" || c.AuthorizationEndpoint != c.Issuer+"/auth/authorize" || c.TokenEndpoint != c.Issuer+"/auth/token" || len(c.Scopes) != 1 || c.Scopes[0] != "agenova/api") {
		return ErrLogin
	}
	issuer, e := url.Parse(c.Issuer)
	if e != nil || issuer.Scheme != "https" || issuer.Host == "" || issuer.User != nil || issuer.RawQuery != "" || issuer.Fragment != "" {
		return ErrLogin
	}
	if c.ClientID == "" || len(c.ClientID) > 128 || c.RedirectURI != Callback || len(c.Scopes) == 0 || len(c.Scopes) > 16 {
		return ErrLogin
	}
	var host string
	for _, raw := range []string{c.AuthorizationEndpoint, c.TokenEndpoint} {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (host != "" && host != u.Host) {
			return ErrLogin
		}
		host = u.Host
	}
	for _, scope := range c.Scopes {
		if scope == "" || len(scope) > 256 || strings.ContainsAny(scope, " \t\r\n") {
			return ErrLogin
		}
	}
	return nil
}
func (c Config) oauth() oauth2.Config {
	return oauth2.Config{ClientID: c.ClientID, RedirectURL: c.RedirectURI, Scopes: c.Scopes, Endpoint: oauth2.Endpoint{AuthURL: c.AuthorizationEndpoint, TokenURL: c.TokenEndpoint, AuthStyle: oauth2.AuthStyleInParams}}
}

type Credentials struct {
	Config Config
	Token  oauth2.Token
}
type Store interface {
	Get(string) (Credentials, error)
	Set(string, Credentials) error
	Delete(string) error
}
type OSStore struct{ ring keyring.Keyring }

func NewOSStore() (Store, error) {
	ring, err := keyring.Open(keyring.Config{ServiceName: "agenova", AllowedBackends: []keyring.BackendType{keyring.KeychainBackend, keyring.WinCredBackend, keyring.SecretServiceBackend}, KeychainSynchronizable: false, KeychainAccessibleWhenUnlocked: true})
	if err != nil {
		return nil, ErrCredentials
	}
	return OSStore{ring}, nil
}
func (s OSStore) Get(server string) (Credentials, error) {
	item, err := s.ring.Get(server)
	if err != nil {
		return Credentials{}, ErrCredentials
	}
	var c Credentials
	if len(item.Data) > 64<<10 || json.Unmarshal(item.Data, &c) != nil || c.Config.Validate() != nil {
		return Credentials{}, ErrCredentials
	}
	return c, nil
}
func (s OSStore) Set(server string, c Credentials) error {
	data, err := json.Marshal(c)
	if err != nil {
		return ErrCredentials
	}
	if err = s.ring.Set(keyring.Item{Key: server, Data: data, Label: "Agenova member login", KeychainNotSynchronizable: true}); err != nil {
		return ErrCredentials
	}
	return nil
}
func (s OSStore) Delete(server string) error {
	err := s.ring.Remove(server)
	if errors.Is(err, keyring.ErrKeyNotFound) {
		return nil
	}
	if err != nil {
		return ErrCredentials
	}
	return nil
}

type Client struct {
	Server      string
	Store       Store
	HTTPClient  *http.Client
	OpenBrowser func(string) error
	mu          sync.Mutex
}

func (c *Client) http() *http.Client {
	result := http.Client{Timeout: 15 * time.Second}
	if c.HTTPClient != nil {
		result = *c.HTTPClient
	}
	result.Timeout = 15 * time.Second
	result.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &result
}
func (c *Client) config(ctx context.Context) (Config, error) {
	if !Origin(c.Server) {
		return Config{}, ErrLogin
	}
	req, err := http.NewRequestWithContext(ctx, "GET", c.Server+"/auth/cli-config", nil)
	if err != nil {
		return Config{}, ErrLogin
	}
	res, err := c.http().Do(req)
	if err != nil {
		return Config{}, ErrLogin
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return Config{}, ErrLogin
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, (16<<10)+1))
	if err != nil || len(data) > 16<<10 {
		return Config{}, ErrLogin
	}
	var config Config
	if json.Unmarshal(data, &config) != nil || config.Validate() != nil {
		return Config{}, ErrLogin
	}
	return config, nil
}
func random() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", ErrLogin
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}
func (c *Client) Login(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Store == nil {
		return ErrCredentials
	}
	config, err := c.config(ctx)
	if err != nil {
		return err
	}
	state, err := random()
	if err != nil {
		return err
	}
	verifier := oauth2.GenerateVerifier()
	listener, err := net.Listen("tcp4", "127.0.0.1:38123")
	if err != nil {
		return errors.New("login callback port 38123 unavailable")
	}
	defer listener.Close()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	result := make(chan string, 1)
	var once sync.Once
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		q := r.URL.Query()
		if r.Method != "GET" || r.Host != "127.0.0.1:38123" || r.URL.Path != "/callback" || len(r.URL.RawQuery) > 8192 || len(q["state"]) != 1 || subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(state)) != 1 {
			http.Error(w, "Invalid login callback.", 400)
			return
		}
		code := q.Get("code")
		if len(q["code"]) != 1 || code == "" || q.Get("error") != "" {
			http.Error(w, "Sign-in was not completed.", 400)
			once.Do(func() { result <- "" })
			return
		}
		consumed := false
		once.Do(func() { result <- code; consumed = true })
		if !consumed {
			http.Error(w, "Login callback already used.", 400)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "You can return to the Agenova CLI to finish signing in.")
	})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, MaxHeaderBytes: 8192}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()
	cfg := config.oauth()
	opener := c.OpenBrowser
	if opener == nil {
		opener = openBrowser
	}
	if err := opener(cfg.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))); err != nil {
		return errors.New("could not open the system browser")
	}
	var code string
	select {
	case code = <-result:
	case <-ctx.Done():
		return errors.New("login timed out or was cancelled")
	}
	if code == "" {
		return ErrLogin
	}
	token, err := cfg.Exchange(context.WithValue(ctx, oauth2.HTTPClient, c.http()), code, oauth2.VerifierOption(verifier))
	if err != nil || token == nil || !token.Valid() || !strings.EqualFold(token.TokenType, "Bearer") || (token.RefreshToken == "" && config.SessionMode != "application") {
		return ErrLogin
	}
	if err := c.check(ctx, token.AccessToken, "/auth/me", "GET"); err != nil {
		return err
	}
	return c.Store.Set(c.Server, Credentials{config, *token})
}
func (c *Client) AccessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !Origin(c.Server) || c.Store == nil {
		return "", ErrCredentials
	}
	saved, err := c.Store.Get(c.Server)
	if err != nil || saved.Config.Validate() != nil {
		return "", ErrCredentials
	}
	cfg := saved.Config.oauth()
	token, err := cfg.TokenSource(context.WithValue(ctx, oauth2.HTTPClient, c.http()), &saved.Token).Token()
	if err != nil || token == nil || !token.Valid() || !strings.EqualFold(token.TokenType, "Bearer") {
		return "", ErrCredentials
	}
	if token.AccessToken != saved.Token.AccessToken || token.RefreshToken != saved.Token.RefreshToken {
		if err := c.Store.Set(c.Server, Credentials{saved.Config, *token}); err != nil {
			return "", err
		}
	}
	return token.AccessToken, nil
}
func (c *Client) check(ctx context.Context, token, path, method string) error {
	req, err := http.NewRequestWithContext(ctx, method, c.Server+path, nil)
	if err != nil {
		return ErrLogin
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := c.http().Do(req)
	if err != nil {
		return ErrLogin
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return ErrLogin
	}
	return nil
}
func (c *Client) Logout(ctx context.Context) error {
	if !Origin(c.Server) || c.Store == nil {
		return ErrCredentials
	}
	token, err := c.AccessToken(ctx)
	if err == nil {
		err = c.check(ctx, token, "/auth/logout", "POST")
	}
	localErr := c.Store.Delete(c.Server)
	if localErr != nil {
		return localErr
	}
	if err != nil {
		return errors.New("local credentials removed; server revocation was not confirmed")
	}
	return nil
}
func openBrowser(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	case "linux":
		cmd = exec.Command("xdg-open", target)
	default:
		return errors.New("unsupported system browser")
	}
	return cmd.Run()
}
