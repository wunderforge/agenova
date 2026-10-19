// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/authentication"
	identitydb "github.com/wunderforge/agenova/internal/authentication/postgres"
	"github.com/wunderforge/agenova/internal/console"
	"github.com/wunderforge/agenova/internal/modelprovider"
	"github.com/wunderforge/agenova/internal/runtime"
	"github.com/wunderforge/agenova/internal/websession"
)

func TestMemberModeCannotFallBack(t *testing.T) {
	for _, tc := range []struct {
		mode, path    string
		present, fail bool
	}{
		{"member", "", false, true}, {"", "", true, true}, {"reference", "/config", true, true},
		{"typo", "", false, true}, {"member", "/config", true, false}, {"", "/config", true, false},
	} {
		t.Run(tc.mode+tc.path, func(t *testing.T) {
			t.Setenv("AGENOVA_AUTH_MODE", tc.mode)
			t.Setenv("AGENOVA_MEMBER_CONFIG", tc.path)
			if !tc.present {
				os.Unsetenv("AGENOVA_MEMBER_CONFIG")
			}
			_, err := memberConfigPath()
			if (err != nil) != tc.fail {
				t.Fatalf("mode result: %v", err)
			}
		})
	}
}
func TestDatabaseTLSAndRedactedErrors(t *testing.T) {
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	good := "postgres://runtime:secret@db.example/agenova?" + url.Values{"sslmode": {"verify-full"}, "sslrootcert": {caFile}, "connect_timeout": {"5"}}.Encode()
	for _, raw := range []string{strings.Replace(good, "verify-full", "disable", 1), strings.Replace(good, "verify-full", "require", 1), good + "&host=evil.example", good + "&sslmode=disable", strings.Replace(good, url.QueryEscape(caFile), "ca.pem", 1)} {
		if secureDatabaseURL(raw) {
			t.Fatal("unsafe DB configuration accepted")
		}
	}
	if !secureDatabaseURL(good) {
		t.Fatal("valid TLS configuration rejected")
	}
	f := filepath.Join(t.TempDir(), "dsn")
	os.WriteFile(f, []byte("postgres://secret:private@localhost/db?sslmode=disable"), 0600)
	_, err := openMemberDatabase(context.Background(), f)
	if err == nil || strings.Contains(err.Error(), "private") {
		t.Fatal("unsafe or unredacted database error")
	}
}
func configFixture(t *testing.T) memberConfig {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>Portal</html>"), 0600); err != nil {
		t.Fatal(err)
	}
	return memberConfig{Origin: "https://app.example", Issuer: "https://cognito-idp.ap-southeast-2.amazonaws.com/ap-southeast-2_Test", OAuthOrigin: "https://team.auth.ap-southeast-2.amazoncognito.com", NativeClientID: "native", WebClientID: "web", Scope: "agenova/api", DatabaseURLFile: filepath.Join(dir, "db"), SessionKeyFile: filepath.Join(dir, "key"), PortalDirectory: dir}
}
func TestStrictMemberConfig(t *testing.T) {
	c := configFixture(t)
	data, _ := json.Marshal(c)
	f := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(f, data, 0600)
	if _, err := loadMemberConfig(f); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{string(data) + "{}", strings.Replace(string(data), `"web"`, `"native"`, 1), strings.Replace(string(data), `"scope"`, `"scpoe"`, 1)} {
		os.WriteFile(f, []byte(bad), 0600)
		if _, err := loadMemberConfig(f); err == nil {
			t.Fatal("ambiguous config accepted")
		}
	}
}

type mustNotRunBackend struct{ runtime.RuntimeBackend }
type mustNotRunExecutor struct{ console.Executor }
type mustNotRunProvider struct{ modelprovider.Client }
type fixtureVerifier struct{ id authentication.Identity }

func (v fixtureVerifier) Verify(_ context.Context, token string) (authentication.Identity, error) {
	if token != "valid" {
		return authentication.Identity{}, authentication.ErrUnauthenticated
	}
	return v.id, nil
}
func TestMemberCompositionProtectsPortalAndAPI(t *testing.T) {
	dsn := os.Getenv("AGENOVA_TEST_POSTGRES")
	if dsn == "" {
		t.Skip("requires disposable PostgreSQL")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "composition_" + time.Now().Format("150405000000000")
	if _, err = admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = checkMemberSchema(context.Background(), db); err == nil {
		t.Fatal("unmigrated database accepted")
	}
	if err = identitydb.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err = identitydb.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err = checkMemberSchema(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	c := configFixture(t)
	store := identitydb.Store{DB: db}
	id := authentication.Identity{Issuer: c.Issuer, Subject: "subject", SessionID: "grant", TokenID: "token", AuthenticatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
	m := identitydb.Member{Issuer: id.Issuer, Subject: id.Subject, Principal: "person", Team: "team", Active: true}
	if err = store.PutMember(context.Background(), "bootstrap", m); err != nil {
		t.Fatal(err)
	}
	boundary, err := authentication.New(fixtureVerifier{id}, store)
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := websession.NewStore(db, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	d := memberDependencies{config: c, db: db, store: store, boundary: boundary, sessions: sessions}
	service, err := console.NewServiceWithOptions(mustNotRunBackend{}, mustNotRunExecutor{}, mustNotRunProvider{}, "", console.Options{
		Access: store, PrepareFor: func([]byte, v0.Principal) (app.PreparedAssignment, error) {
			return app.PreparedAssignment{}, errors.New("must not submit")
		},
		Setup: func() (console.Setup, error) { return console.Setup{}, nil }, Configure: func(*v0.AgentTemplate) (app.ResolvedLaunch, error) {
			return app.ResolvedLaunch{}, errors.New("must not configure")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	h, err := d.handler(service)
	if err != nil {
		t.Fatal(err)
	}
	call := func(path, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", c.Origin+path, nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := call("/", ""); w.Code != 303 || w.Header().Get("Location") != "/login" {
		t.Fatal("root did not require login")
	}
	if w := call("/api/requests", ""); w.Code != 401 {
		t.Fatal("anonymous API exposed")
	}
	if w := call("/?mode=connected", "valid"); w.Code != 200 || !strings.Contains(w.Body.String(), "Portal") {
		t.Fatalf("Portal: %d %s", w.Code, w.Body)
	}
	if w := call("/api/requests", "valid"); w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("API: %d %s", w.Code, w.Body)
	}
	if w := call("/api/provider-observations", "valid"); w.Code != 403 {
		t.Fatal("operator diagnostics exposed")
	}
	// Management routes require current explicit duties, not just a valid login.
	t.Setenv("AGENOVA_PLATFORM_NAME", "sydney")
	post := func(path, body string) int {
		r := httptest.NewRequest("POST", c.Origin+path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer valid")
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if code := post("/api/platform/authorize", `{"installation":"sydney","action":"apply"}`); code != 403 {
		t.Fatal("implicit platform access", code)
	}
	if _, err = db.Exec(`INSERT INTO agenova_scoped_roles VALUES('person','team','platform-operator','installation','sydney')`); err != nil {
		t.Fatal(err)
	}
	if code := post("/api/platform/authorize", `{"installation":"sydney","action":"apply"}`); code != 204 {
		t.Fatal("scoped platform authorization", code)
	}
	if code := post("/api/platform/authorize", `{"installation":"other","action":"apply"}`); code != 403 {
		t.Fatal("foreign installation", code)
	}
	if code := post("/api/platform/authorize", `{"installation":"sydney","action":"destroy"}`); code != 400 {
		t.Fatal("unknown action", code)
	}
	if _, err = db.Exec(`DELETE FROM agenova_scoped_roles WHERE principal='person'`); err != nil {
		t.Fatal(err)
	}
	if code := post("/api/platform/authorize", `{"installation":"sydney","action":"status"}`); code != 403 {
		t.Fatal("revoked operator", code)
	}
	if _, err = db.Exec(`UPDATE agenova_members SET account_role='admin' WHERE principal='person'; INSERT INTO agenova_projects VALUES('team','operator-project'); INSERT INTO agenova_project_grants VALUES('person','team','operator-project','admin')`); err != nil {
		t.Fatal(err)
	}
	if code := post("/api/platform/authorize", `{"installation":"sydney","action":"apply"}`); code != 204 {
		t.Fatal("team admin deployment", code)
	}
	if code := post("/api/platform/authorize", `{"installation":"other","action":"apply"}`); code != 403 {
		t.Fatal("team admin installation escape", code)
	}
	if _, err = db.Exec(`UPDATE agenova_members SET account_role='member' WHERE principal='person'`); err != nil {
		t.Fatal(err)
	}
	if code := post("/api/platform/authorize", `{"installation":"sydney","action":"apply"}`); code != 403 {
		t.Fatal("demoted admin deployment", code)
	}
	m.Active = false
	if err = store.PutMember(context.Background(), "operator", m); err != nil {
		t.Fatal(err)
	}
	if w := call("/api/requests", "valid"); w.Code == 200 {
		t.Fatal("disabled member retained access")
	}
	if _, err = db.Exec(`UPDATE agenova_schema_migrations SET checksum='changed'`); err != nil {
		t.Fatal(err)
	}
	if err = identitydb.Migrate(context.Background(), db); err == nil {
		t.Fatal("migration rewrite accepted")
	}
}

func TestMemberReadinessKeepsKubeletAccessWithoutStatusExposure(t *testing.T) {
	for _, tc := range []struct {
		path, remote string
		want         int
	}{
		{"/readyz", "10.0.0.2:3456", 200}, {"/v1/status", "10.0.0.2:3456", 404}, {"/v1/status", "127.0.0.1:3456", 200},
	} {
		r := httptest.NewRequest("GET", "http://pod"+tc.path, nil)
		r.RemoteAddr = tc.remote
		r.Header.Set("X-Forwarded-For", "127.0.0.1")
		w := httptest.NewRecorder()
		memberStatusHandler().ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("%s %s: %d", tc.remote, tc.path, w.Code)
		}
	}
}

func TestGoogleMemberConfigRejectsMixedProviders(t *testing.T) {
	c := configFixture(t)
	c.Issuer = ""
	c.OAuthOrigin = ""
	c.NativeClientID = ""
	c.WebClientID = ""
	c.Scope = ""
	c.GoogleOAuthFile = filepath.Join(t.TempDir(), "google.json")
	credentials := map[string]string{"provider": "google", "project_id": "test-project", "client_id": "123-test.apps.googleusercontent.com", "client_secret": "test-only-secret", "redirect_uri": c.Origin + "/auth/callback"}
	data, _ := json.Marshal(credentials)
	if err := os.WriteFile(c.GoogleOAuthFile, data, 0600); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "member.json")
	save := func() {
		data, _ := json.Marshal(c)
		if err := os.WriteFile(f, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	save()
	if _, err := loadMemberConfig(f); err != nil {
		t.Fatal("Google config rejected", err)
	}
	c.Issuer = "https://cognito-idp.ap-southeast-2.amazonaws.com/ap-southeast-2_Test"
	save()
	if _, err := loadMemberConfig(f); err == nil {
		t.Fatal("mixed identity providers accepted")
	}
	c.Issuer = ""
	save()
	credentials["redirect_uri"] = "https://evil.example/auth/callback"
	data, _ = json.Marshal(credentials)
	os.WriteFile(c.GoogleOAuthFile, data, 0600)
	if _, err := loadMemberConfig(f); err == nil || strings.Contains(err.Error(), "test-only-secret") {
		t.Fatal("wrong callback or secret disclosure")
	}
}
