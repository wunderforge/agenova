// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// oidcfixture serves disposable JWKS and issues two synthetic test identities.
// It is for installed-kind smoke only; it is not a company identity provider.
package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"time"
)

const fixtureIssuer = "https://oidc-fixture.agenova.invalid"

func main() {
	outputRoot := flag.String("output-root", os.TempDir(), "parent for disposable protected token files")
	flag.Parse()
	if err := run(*outputRoot); err != nil {
		log.Fatal(err)
	}
}

func run(outputRoot string) error {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("create fixture signing key: %w", err)
	}
	dir, err := os.MkdirTemp(outputRoot, "agenova-oidc-fixture-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}
	config := map[string]any{
		"issuer": fixtureIssuer, "audience": "agenova", "jwksUrl": "http://host.docker.internal:18080/keys",
		"teamClaim": "groups", "teamMappings": map[string]string{"engineering": "team-a", "support": "team-b"},
		"authenticationContextClaim": "acr",
	}
	if err := writeJSON(dir+"/oidc.json", config); err != nil {
		return err
	}
	for _, user := range []struct{ subject, group string }{{"user-a", "engineering"}, {"user-b", "support"}} {
		token, err := sign(key, map[string]any{
			"iss": fixtureIssuer, "aud": "agenova", "sub": user.subject, "groups": user.group,
			"acr": "fixture-mfa", "nbf": time.Now().Add(-time.Minute).Unix(), "exp": time.Now().Add(time.Hour).Unix(),
		})
		if err != nil {
			return err
		}
		if err := os.WriteFile(dir+"/"+user.subject+".token", []byte(token+"\n"), 0600); err != nil {
			return err
		}
	}
	modulus := base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes())
	exponent := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes())
	keys, err := json.Marshal(map[string]any{"keys": []any{map[string]string{"kty": "RSA", "kid": "fixture", "alg": "RS256", "use": "sig", "n": modulus, "e": exponent}}})
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /keys", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(keys)
	})
	fmt.Printf("fixture_dir=%s\n", dir)
	fmt.Printf("jwks_url=http://host.docker.internal:18080/keys\n")
	return http.ListenAndServe(":18080", mux)
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0600)
}

func sign(key *rsa.PrivateKey, claims map[string]any) (string, error) {
	header, err := json.Marshal(map[string]string{"alg": "RS256", "kid": "fixture", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	message := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	hash := sha256.Sum256([]byte(message))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		return "", err
	}
	return message + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}
