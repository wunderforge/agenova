// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// setInClusterKubeconfig writes only service-account *paths*, never token
// material, to a private kubeconfig for the bundled kubectl adapter.
func setInClusterKubeconfig() error {
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if net.ParseIP(host) == nil || port == "" {
		return fmt.Errorf("in-cluster Kubernetes API coordinates are unavailable")
	}
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token"); err != nil {
		return fmt.Errorf("Kubernetes service account is unavailable")
	}
	server := "https://" + net.JoinHostPort(host, port)
	config := strings.Join([]string{
		"apiVersion: v1", "kind: Config", "clusters:", "- cluster:",
		"    certificate-authority: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
		"    server: " + server, "  name: in-cluster", "contexts:",
		"- context:", "    cluster: in-cluster", "    user: agenova-control-plane",
		"  name: in-cluster", "current-context: in-cluster", "users:",
		"- name: agenova-control-plane", "  user:",
		"    tokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token", "",
	}, "\n")
	path := filepath.Join(os.TempDir(), "agenova-in-cluster-kubeconfig")
	if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
		return fmt.Errorf("prepare in-cluster Kubernetes client: %w", err)
	}
	return os.Setenv("KUBECONFIG", path)
}
