// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// agenova-control-plane is the minimum internal reference-install process. It
// intentionally exposes only readiness and secret-free installation status.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

type status struct {
	Platform       string `json:"platform"`
	Revision       string `json:"revision"`
	Policy         string `json:"policy"`
	State          string `json:"state"`
	ReadinessScope string `json:"readinessScope"`
}

func main() {
	server := &http.Server{Addr: ":8080", Handler: handler(), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("agenova reference control plane listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}

func handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /readyz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("ready\n"))
	})
	mux.HandleFunc("GET /v1/status", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(status{Platform: os.Getenv("AGENOVA_PLATFORM_NAME"), Revision: os.Getenv("AGENOVA_PLATFORM_REVISION"), Policy: os.Getenv("AGENOVA_POLICY_REF"), State: "available", ReadinessScope: "installation-components"})
	})
	return mux
}
