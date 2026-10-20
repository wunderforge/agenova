// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/authentication/cognito"
	"github.com/wunderforge/agenova/internal/authentication/googleoidc"
	identitydb "github.com/wunderforge/agenova/internal/authentication/postgres"
	"github.com/wunderforge/agenova/internal/console"
	"github.com/wunderforge/agenova/internal/invitations"
	"github.com/wunderforge/agenova/internal/landing"
	"github.com/wunderforge/agenova/internal/login"
	"github.com/wunderforge/agenova/internal/websession"
	"golang.org/x/oauth2"
)

// Operator configuration contains file references, never plaintext credentials.
// Its presence selects authenticated mode. Invalid or partial config is fatal.
type memberConfig struct {
	RegistrationTeam string `json:"registrationTeam,omitempty"`
	GoogleOAuthFile  string `json:"googleOAuthFile"`
	Origin           string `json:"origin"`
	Issuer           string `json:"issuer"`
	OAuthOrigin      string `json:"oauthOrigin"`
	NativeClientID   string `json:"nativeClientId"`
	WebClientID      string `json:"webClientId"`
	Scope            string `json:"scope"`
	DatabaseURLFile  string `json:"databaseUrlFile"`
	SessionKeyFile   string `json:"sessionKeyFile"`
	PortalDirectory  string `json:"portalDirectory"`
}

func readBoundedFile(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("required configuration file unavailable")
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil || !stat.Mode().IsRegular() || stat.Size() > limit {
		return nil, errors.New("invalid configuration file")
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, errors.New("invalid configuration file")
	}
	return data, nil
}

func loadMemberConfig(path string) (memberConfig, error) {
	var c memberConfig
	b, err := readBoundedFile(path, 32<<10)
	if err != nil {
		return c, err
	}
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.DisallowUnknownFields()
	if d.Decode(&c) != nil || d.Decode(new(any)) != io.EOF {
		return c, errors.New("invalid member service configuration")
	}
	if !login.Origin(c.Origin) || !filepath.IsAbs(c.DatabaseURLFile) || !filepath.IsAbs(c.SessionKeyFile) || !filepath.IsAbs(c.PortalDirectory) {
		return c, errors.New("complete member configuration required")
	}
	if c.RegistrationTeam != "" && (c.GoogleOAuthFile == "" || len(c.RegistrationTeam) > 128) {
		return c, errors.New("registration requires Google and a bounded team")
	}
	if c.GoogleOAuthFile != "" {
		if !filepath.IsAbs(c.GoogleOAuthFile) || c.Issuer != "" || c.OAuthOrigin != "" || c.NativeClientID != "" || c.WebClientID != "" || c.Scope != "" {
			return c, errors.New("Google and legacy identity configuration cannot be combined")
		}
		if _, err := loadGoogleClient(c); err != nil {
			return c, err
		}
	} else {
		if !login.Origin(c.OAuthOrigin) || c.NativeClientID == c.WebClientID {
			return c, errors.New("complete legacy identity configuration required")
		}
		if _, err := cognito.New(cognito.Config{Issuer: c.Issuer, ClientIDs: []string{c.NativeClientID, c.WebClientID}, RequiredScope: c.Scope}); err != nil {
			return c, err
		}
	}
	if _, err := readBoundedFile(filepath.Join(c.PortalDirectory, "index.html"), 1<<20); err != nil {
		return c, errors.New("Portal build unavailable")
	}
	return c, nil
}

// Google client secret is read only from a private operator-mounted file.
func loadGoogleClient(c memberConfig) (*googleoidc.Client, error) {
	var credentials struct {
		Provider     string `json:"provider"`
		ProjectID    string `json:"project_id"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		RedirectURI  string `json:"redirect_uri"`
	}
	b, err := readBoundedFile(c.GoogleOAuthFile, 16<<10)
	if err != nil {
		return nil, err
	}
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.DisallowUnknownFields()
	if d.Decode(&credentials) != nil || d.Decode(new(any)) != io.EOF || credentials.Provider != "google" || credentials.RedirectURI != c.Origin+"/auth/callback" {
		return nil, errors.New("invalid Google login configuration")
	}
	return googleoidc.NewClient(credentials.ClientID, credentials.ClientSecret, credentials.RedirectURI)
}

// Require hostname-verified TLS, explicit CA and a pinned database endpoint.
// Never log a driver error: it may contain the supplied connection credentials.
func secureDatabaseURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "postgres" || u.Hostname() == "" || u.User == nil || u.User.Username() == "" || u.Path == "" || u.Fragment != "" {
		return false
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return false
	}
	for k, v := range q {
		if len(v) != 1 {
			return false
		}
		switch k {
		case "sslmode", "sslrootcert", "connect_timeout":
		default:
			return false
		}
	}
	return q.Get("sslmode") == "verify-full" && filepath.IsAbs(q.Get("sslrootcert")) && q.Get("connect_timeout") == "5"
}
func openMemberDatabase(ctx context.Context, path string) (*sql.DB, error) {
	b, err := readBoundedFile(path, 16<<10)
	if err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(string(b))
	if !secureDatabaseURL(raw) {
		return nil, errors.New("database requires explicit verify-full TLS, CA file and five-second connection timeout")
	}
	db, err := sql.Open("postgres", raw)
	if err != nil {
		return nil, errors.New("database configuration rejected")
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if db.PingContext(ctx) != nil {
		db.Close()
		return nil, errors.New("member database unavailable")
	}
	return db, nil
}

// The runtime checks schema availability but cannot create or alter tables.
func checkMemberSchema(ctx context.Context, db *sql.DB) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := identitydb.CheckMigration(ctx, db); err != nil {
		return err
	}
	for _, query := range []string{
		"SELECT issuer,subject,principal,team,active,account_role,login_after FROM agenova_members LIMIT 0",
		"SELECT id,issuer,subject,email,team,status,principal FROM agenova_access_requests LIMIT 0",
		"SELECT issuer,subject,session_id FROM agenova_revocations LIMIT 0",
		"SELECT team,project FROM agenova_projects LIMIT 0",
		"SELECT principal,team,project,role FROM agenova_project_grants LIMIT 0",
		"SELECT id_hash,payload,expires_at,version FROM agenova_browser_sessions LIMIT 0",
		"SELECT id_hash,payload,expires_at FROM agenova_login_attempts LIMIT 0",
		"SELECT actor,operation,principal FROM agenova_identity_audit LIMIT 0",
	} {
		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			return errors.New("member schema unavailable; run the explicit operator migration")
		}
		rows.Close()
	}
	return nil
}

type memberDependencies struct {
	google    *googleoidc.Client
	config    memberConfig
	db        *sql.DB
	store     identitydb.Store
	boundary  *authentication.Boundary
	sessions  *websession.Store
	directory invitations.Directory
}

func openMemberDependencies(ctx context.Context, path string) (*memberDependencies, error) {
	c, err := loadMemberConfig(path)
	if err != nil {
		return nil, err
	}
	keyData, err := readBoundedFile(c.SessionKeyFile, 256)
	if err != nil {
		return nil, err
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(keyData)))
	if err != nil || len(key) != 32 {
		return nil, errors.New("session key must be a base64-encoded 32-byte secret")
	}
	db, err := openMemberDatabase(ctx, c.DatabaseURLFile)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			db.Close()
		}
	}()
	if err := checkMemberSchema(ctx, db); err != nil {
		return nil, err
	}
	store := identitydb.Store{DB: db}
	var verifier authentication.Verifier
	var memberships authentication.Memberships = store
	var google *googleoidc.Client
	if c.GoogleOAuthFile != "" {
		google, err = loadGoogleClient(c)
		verifier = identitydb.ApplicationSessions{Store: store}
	} else {
		verifier, err = cognito.New(cognito.Config{Issuer: c.Issuer, ClientIDs: []string{c.NativeClientID, c.WebClientID}, RequiredScope: c.Scope})
		memberships = identitydb.InvitedMemberships{Store: store}
	}
	if err != nil {
		return nil, err
	}
	boundary, err := authentication.New(verifier, memberships)
	if err != nil {
		return nil, err
	}
	sessions, err := websession.NewStore(db, key)
	if err != nil {
		return nil, err
	}
	if google != nil {
		ok = true
		return &memberDependencies{config: c, db: db, store: store, boundary: boundary, sessions: sessions, google: google}, nil
	}
	awsConfig, err := config.LoadDefaultConfig(ctx, config.WithRegion(strings.Split(strings.TrimPrefix(c.Issuer, "https://cognito-idp."), ".")[0]))
	if err != nil {
		return nil, errors.New("directory configuration unavailable")
	}
	directory := cognito.Directory{PoolID: c.Issuer[strings.LastIndex(c.Issuer, "/")+1:], Client: cognitoidentityprovider.NewFromConfig(awsConfig, func(o *cognitoidentityprovider.Options) { o.RetryMaxAttempts = 1 })}
	ok = true
	return &memberDependencies{config: c, db: db, store: store, boundary: boundary, sessions: sessions, directory: directory}, nil
}
func (d *memberDependencies) handler(service *console.Service) (http.Handler, error) {
	api, err := console.AuthenticatedHandler(service, d.boundary)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	invitationService := invitations.Service{Registration: d.config.RegistrationTeam != "", Issuer: d.config.Issuer, Directory: d.directory, Store: d.store, Google: d.google != nil, Origin: d.config.Origin}
	mux.Handle("/admin", invitations.Page(invitationService, d.boundary))
	mux.HandleFunc("/members", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/admin", http.StatusSeeOther) })
	mux.Handle("/api/invitations", invitations.Handler(invitationService, d.boundary))
	mux.Handle("/api/platform/authorize", d.platformAuthorizationHandler())
	mux.Handle("/api/governance", d.governanceHandler())
	if d.config.RegistrationTeam != "" {
		mux.Handle("/api/access-requests", invitations.AccessRequestHandler(d.store, d.boundary, os.Getenv("AGENOVA_PLATFORM_NAME")))
	}
	mux.Handle("/api/members", invitations.MemberHandler(d.store, d.boundary, os.Getenv("AGENOVA_PLATFORM_NAME")))
	if d.google != nil {
		mux.Handle("/api/invitations/manage", invitations.ManageHandler(d.store, d.boundary, d.config.Origin))
	}
	mux.Handle("/api/", api)
	// Portal uses hash routes. Only its built assets are served; no directory lists.
	assets := http.FileServer(http.Dir(d.config.PortalDirectory))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		if r.URL.Path == "/index.html" || (r.URL.Path == "/" && r.URL.Query().Get("mode") != "connected") {
			http.Redirect(w, r, "/?mode=connected", 303)
			return
		}
		path := filepath.Join(d.config.PortalDirectory, filepath.FromSlash(r.URL.Path))
		info, err := os.Stat(path)
		if err != nil || (info.IsDir() && r.URL.Path != "/") {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			index, err := readBoundedFile(filepath.Join(d.config.PortalDirectory, "index.html"), 1<<20)
			if err != nil {
				http.Error(w, "Portal unavailable", 503)
				return
			}
			accountNavigation := "member"
			if p, err := d.boundary.Authenticate(r); err == nil {
				if _, err = d.store.ListInvitations(r.Context(), p); err == nil {
					accountNavigation = "admin"
				}
			}
			marker := `<meta name="agenova-account-navigation" content="` + accountNavigation + `">`
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, strings.Replace(string(index), "</head>", marker+"</head>", 1))
			return
		}
		assets.ServeHTTP(w, r)
	}))
	c := d.config
	if d.google != nil {
		var registrar websession.AccessRegistrar
		if c.RegistrationTeam != "" {
			registrar = d.store
		}
		return websession.New(websession.Config{Registration: registrar, RegistrationTeam: c.RegistrationTeam, Origin: c.Origin, Google: d.google, ApplicationSessions: d.store, Invitations: d.store, Store: d.sessions, Boundary: d.boundary, Revoker: d.store, Next: mux, Landing: landing.New(c.PortalDirectory)})
	}
	scopes := []string{"openid", "email", c.Scope}
	native := login.Config{Issuer: c.Issuer, ClientID: c.NativeClientID, AuthorizationEndpoint: c.OAuthOrigin + "/oauth2/authorize", TokenEndpoint: c.OAuthOrigin + "/oauth2/token", Scopes: scopes, RedirectURI: login.Callback}
	return websession.New(websession.Config{Origin: c.Origin, Native: native, Web: oauth2.Config{ClientID: c.WebClientID, RedirectURL: c.Origin + "/auth/callback", Scopes: scopes, Endpoint: oauth2.Endpoint{AuthURL: native.AuthorizationEndpoint, TokenURL: native.TokenEndpoint, AuthStyle: oauth2.AuthStyleInParams}}, Store: d.sessions, Boundary: d.boundary, Revoker: d.store, Next: mux, Landing: landing.New(c.PortalDirectory)})
}

func bootstrapOwnerCommand(args []string) error {
	if len(args) != 5 || args[1] != "--config-file" || args[3] != "--owner-file" {
		return errors.New("identity-bootstrap requires --config-file <member config> --owner-file <private owner input>")
	}
	var owner struct {
		Email   string `json:"email"`
		Team    string `json:"team"`
		Project string `json:"project"`
	}
	data, err := readBoundedFile(args[4], 2048)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&owner) != nil || decoder.Decode(new(any)) != io.EOF {
		return errors.New("invalid owner input")
	}
	d, err := openMemberDependencies(context.Background(), args[2])
	if err != nil {
		return err
	}
	defer d.db.Close()
	if d.google != nil {
		_, proof, err := d.store.ReserveGoogleOwner(context.Background(), owner.Team, invitations.Request{Email: owner.Email, Project: owner.Project, Role: "admin"})
		if err != nil {
			return err
		}
		// Invitation proof goes to a new protected operator file, never stdout/logs.
		output := args[4] + ".invitation"
		f, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return errors.New("invitation output unavailable; inspect pending bootstrap before retrying")
		}
		_, err = f.WriteString(d.config.Origin + "/login#invite=" + proof + "\n")
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			return errors.New("invitation write failed; inspect pending bootstrap")
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "pending", "invitationFile": output})
	}
	service := invitations.Service{Issuer: d.config.Issuer, Directory: d.directory, Store: d.store}
	invite, err := service.BootstrapOwner(context.Background(), owner.Team, invitations.Request{Email: owner.Email, Project: owner.Project, Role: "admin"})
	if err != nil {
		return err
	}
	// No email, directory subject, temporary password or credential is printed.
	return json.NewEncoder(os.Stdout).Encode(map[string]string{"invitationId": invite.ID, "status": invite.Status})
}
