// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	version := os.Getenv("PAYMENT_VERSION")
	if version == "" {
		log.Fatal("PAYMENT_VERSION is required")
	}
	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok\n")) })
	http.HandleFunc("/version", func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprintln(w, version) })
	log.Fatal(http.ListenAndServe(":8080", nil))
}
