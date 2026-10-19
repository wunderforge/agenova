// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package login

import (
	"golang.org/x/oauth2"
	"os"
	"testing"
	"time"
)

// Opt-in real OS storage test; never touches any user's Agenova origin.
func TestNativeCredentialStoreRoundTrip(t *testing.T) {
	if os.Getenv("AGENOVA_TEST_OS_STORE") != "1" {
		t.Skip("requires disposable unlocked native credential store")
	}
	server := "https://credential-test-" + time.Now().Format("20060102150405.000000000") + ".example"
	store, err := NewOSStore()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Delete(server)
	credentials := Credentials{Config: Config{SessionMode: "application", Issuer: server, ClientID: "agenova-cli", AuthorizationEndpoint: server + "/auth/authorize", TokenEndpoint: server + "/auth/token", Scopes: []string{"agenova/api"}, RedirectURI: Callback}, Token: oauth2.Token{AccessToken: "synthetic-access", TokenType: "Bearer", RefreshToken: "synthetic-refresh"}}
	if err = store.Set(server, credentials); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewOSStore()
	if err != nil {
		t.Fatal(err)
	}
	got, err := reopened.Get(server)
	if err != nil || got.Token.AccessToken != credentials.Token.AccessToken || got.Token.RefreshToken != credentials.Token.RefreshToken {
		t.Fatal("native persistence failed", err)
	}
	if err = reopened.Delete(server); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Get(server); err == nil {
		t.Fatal("deleted credential still readable")
	}
}
