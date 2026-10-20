// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package bundled

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"github.com/wunderforge/agenova/internal/platformapply"
	"reflect"
	"sort"
	"strings"
)

//go:embed observability/*
var observationAssets embed.FS

func asset(name string) string {
	b, err := observationAssets.ReadFile("observability/" + name)
	if err != nil {
		panic(err)
	}
	return string(b)
}
func stackSteps(request platformapply.DeploymentRequest, namespace string) []manifestStep {
	config, enabled := stackConfig(request)
	if !enabled {
		return nil
	}
	class := config["storageClass"].(string)
	metadata := func(name string) map[string]any {
		return map[string]any{"name": name, "namespace": namespace, "labels": managedLabels(), "annotations": map[string]any{"agenova.io/platform-revision": request.Platform.Revision}}
	}
	steps := []manifestStep{}
	if proxy, ok := config["grafanaAuthProxy"].(map[string]any); ok {
		steps = append(steps, manifestStep{name: "agenova-grafana-private", category: "observability", object: map[string]any{"apiVersion": "networking.k8s.io/v1", "kind": "NetworkPolicy", "metadata": metadata("agenova-grafana-private"), "spec": map[string]any{"podSelector": map[string]any{"matchLabels": map[string]any{"app.kubernetes.io/name": "agenova-grafana"}}, "policyTypes": []any{"Ingress"}, "ingress": []any{map[string]any{"from": []any{map[string]any{"namespaceSelector": map[string]any{"matchLabels": map[string]any{"kubernetes.io/metadata.name": proxy["namespace"]}}, "podSelector": map[string]any{"matchLabels": map[string]any{"app": proxy["app"]}}}}, "ports": []any{map[string]any{"protocol": "TCP", "port": 3000}}}}}}})
	}

	pvc := func(name, size string) {
		steps = append(steps, manifestStep{name: name + "-data", category: "observability", object: map[string]any{"apiVersion": "v1", "kind": "PersistentVolumeClaim", "metadata": metadata(name + "-data"), "spec": map[string]any{"accessModes": []any{"ReadWriteOnce"}, "storageClassName": class, "resources": map[string]any{"requests": map[string]any{"storage": size}}}}})
	}
	pvc("agenova-evidence", "1Gi")
	components := []struct {
		name, image, config string
		port                int
		args                []any
		ready               string
		data                bool
	}{
		{"otel", "otel/opentelemetry-collector-contrib:0.116.1", "collector.yaml", 13133, []any{"--config=/etc/observation/config.yaml"}, "/", false},
		{"tempo", "grafana/tempo:2.6.1", "tempo.yaml", 3200, []any{"-config.file=/etc/observation/config.yaml"}, "/ready", true},
		{"loki", "grafana/loki:3.3.2", "loki.yaml", 3100, []any{"-config.file=/etc/observation/config.yaml"}, "/ready", true},
		{"prometheus", "prom/prometheus:v3.0.1", "prometheus.yaml", 9090, []any{"--config.file=/etc/observation/config.yaml", "--storage.tsdb.path=/data", "--storage.tsdb.retention.time=7d"}, "/-/ready", true},
		{"grafana", "grafana/grafana:11.4.0", "", 3000, nil, "/api/health", true},
	}
	for _, c := range components {
		name := "agenova-" + c.name
		labels := map[string]any{"app.kubernetes.io/name": name, "app.kubernetes.io/managed-by": "agenova"}
		data := map[string]any{}
		if c.config != "" {
			data["config.yaml"] = asset(c.config)
		} else {
			data["datasources.yaml"] = asset("datasources.yaml")
			data["dashboards.yaml"] = asset("dashboards.yaml")
			data["dashboard.json"] = asset("dashboard.json")
		}
		digestBytes, _ := json.Marshal(data)
		digest := fmt.Sprintf("%x", sha256.Sum256(digestBytes))
		steps = append(steps, manifestStep{name: name + "-config", category: "observability", object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": metadata(name + "-config"), "data": data}})
		mounts := []any{map[string]any{"name": "config", "mountPath": "/etc/observation", "readOnly": true}}
		volumes := []any{map[string]any{"name": "config", "configMap": map[string]any{"name": name + "-config"}}}
		if c.data {
			pvc(name, "2Gi")
			path := "/data"
			if c.name == "grafana" {
				path = "/var/lib/grafana"
			}
			mounts = append(mounts, map[string]any{"name": "data", "mountPath": path})
			volumes = append(volumes, map[string]any{"name": "data", "persistentVolumeClaim": map[string]any{"claimName": name + "-data"}})
		}
		ports := []any{map[string]any{"name": "http", "containerPort": c.port}}
		servicePorts := []any{map[string]any{"name": "http", "port": c.port, "targetPort": c.port}}
		if c.name == "otel" || c.name == "tempo" {
			ports = append(ports, map[string]any{"name": "otlp", "containerPort": 4318})
			servicePorts = append(servicePorts, map[string]any{"name": "otlp", "port": 4318, "targetPort": 4318})
		}
		if c.name == "otel" {
			for i, port := range []int{8888, 8889} {
				servicePorts = append(servicePorts, map[string]any{"name": fmt.Sprintf("metrics%d", i), "port": port, "targetPort": port})
			}
		}
		container := map[string]any{"name": c.name, "image": c.image, "imagePullPolicy": "IfNotPresent", "args": c.args, "ports": ports, "volumeMounts": mounts, "resources": map[string]any{"requests": map[string]any{"cpu": "50m", "memory": "128Mi"}, "limits": map[string]any{"cpu": "1", "memory": "512Mi"}}, "readinessProbe": map[string]any{"httpGet": map[string]any{"path": c.ready, "port": c.port}, "initialDelaySeconds": 2, "periodSeconds": 3}}
		if c.name == "grafana" {
			container["env"] = []any{map[string]any{"name": "GF_SECURITY_ADMIN_USER", "valueFrom": map[string]any{"secretKeyRef": map[string]any{"name": config["grafanaAdminSecretRef"], "key": "username"}}}, map[string]any{"name": "GF_SECURITY_ADMIN_PASSWORD", "valueFrom": map[string]any{"secretKeyRef": map[string]any{"name": config["grafanaAdminSecretRef"], "key": "password"}}}, map[string]any{"name": "GF_ANALYTICS_REPORTING_ENABLED", "value": "false"}, map[string]any{"name": "GF_ANALYTICS_CHECK_FOR_UPDATES", "value": "false"}}
			if proxy, ok := config["grafanaAuthProxy"].(map[string]any); ok {
				values := map[string]string{"GF_SERVER_ROOT_URL": proxy["publicOrigin"].(string) + "/grafana/", "GF_SERVER_SERVE_FROM_SUB_PATH": "true", "GF_SECURITY_COOKIE_SECURE": "true", "GF_AUTH_PROXY_ENABLED": "true", "GF_AUTH_PROXY_HEADER_NAME": "X-WEBAUTH-USER", "GF_AUTH_PROXY_HEADER_PROPERTY": "username", "GF_AUTH_PROXY_HEADERS": "Role:X-WEBAUTH-ROLE", "GF_AUTH_PROXY_ENABLE_LOGIN_TOKEN": "false", "GF_AUTH_PROXY_AUTO_SIGN_UP": "true", "GF_AUTH_PROXY_SYNC_TTL": "0", "GF_AUTH_ANONYMOUS_ENABLED": "false", "GF_AUTH_BASIC_ENABLED": "false", "GF_AUTH_DISABLE_LOGIN_FORM": "true", "GF_USERS_AUTO_ASSIGN_ORG_ROLE": "Viewer", "GF_USERS_ALLOW_SIGN_UP": "false"}
				names := make([]string, 0, len(values))
				for name := range values {
					names = append(names, name)
				}
				sort.Strings(names)
				for _, name := range names {
					container["env"] = append(container["env"].([]any), map[string]any{"name": name, "value": values[name]})
				}
			}
			mounts = append(mounts, map[string]any{"name": "config", "mountPath": "/etc/grafana/provisioning/datasources/datasources.yaml", "subPath": "datasources.yaml", "readOnly": true}, map[string]any{"name": "config", "mountPath": "/etc/grafana/provisioning/dashboards/dashboards.yaml", "subPath": "dashboards.yaml", "readOnly": true}, map[string]any{"name": "config", "mountPath": "/var/lib/grafana/dashboards/agenova.json", "subPath": "dashboard.json", "readOnly": true})
			container["volumeMounts"] = mounts
		}
		steps = append(steps, manifestStep{name: name, category: "observability", object: map[string]any{"apiVersion": "apps/v1", "kind": "Deployment", "metadata": metadata(name), "spec": map[string]any{"replicas": 1, "strategy": map[string]any{"type": "Recreate"}, "selector": map[string]any{"matchLabels": labels}, "template": map[string]any{"metadata": map[string]any{"labels": labels, "annotations": map[string]any{"agenova.io/platform-revision": request.Platform.Revision, "agenova.io/config-digest": digest}}, "spec": map[string]any{"automountServiceAccountToken": false, "securityContext": map[string]any{"fsGroup": 1000}, "volumes": volumes, "containers": []any{container}}}}}})
		steps = append(steps, manifestStep{name: name + "-service", category: "observability", object: map[string]any{"apiVersion": "v1", "kind": "Service", "metadata": metadata(name), "spec": map[string]any{"type": "ClusterIP", "selector": labels, "ports": servicePorts}}})
	}
	return steps
}
func stackKind(step manifestStep) string { return strings.ToLower(step.object["kind"].(string)) }
func stackMatches(actual map[string]any, step manifestStep, revision string) bool {
	if actual == nil {
		return false
	}
	data, _ := json.Marshal(step.object)
	var desired map[string]any
	json.Unmarshal(data, &desired)
	switch stackKind(step) {
	case "deployment":
		return deploymentMatches(actual, step.object, revision)
	case "configmap":
		return reflect.DeepEqual(actual["data"], step.object["data"])
	case "service":
		a, _ := actual["spec"].(map[string]any)
		b, _ := desired["spec"].(map[string]any)
		return reflect.DeepEqual(a["selector"], b["selector"]) && expectedFieldsMatch(a, b)
	case "networkpolicy":
		return reflect.DeepEqual(actual["spec"], desired["spec"])
	case "persistentvolumeclaim":
		return expectedFieldsMatch(actual["spec"], desired["spec"])
	}
	return false
}
func (k *KubernetesDeployment) planStack(ctx context.Context, request platformapply.DeploymentRequest, contextName, namespace string) ([]platformapply.Change, []platformapply.ComponentStatus, error) {
	var changes []platformapply.Change
	var statuses []platformapply.ComponentStatus
	for _, step := range stackSteps(request, namespace) {
		kind := stackKind(step)
		actual, err := k.getJSON(ctx, contextName, namespace, kind, objectName(step.object))
		if err != nil && !isNotFound(err) {
			return nil, nil, err
		}
		if err = requireManaged(actual, kind, objectName(step.object)); err != nil {
			return nil, nil, err
		}
		ready := stackMatches(actual, step, request.Platform.Revision)
		// A PVC with WaitForFirstConsumer is configured before its Pod is scheduled.
		// Deployment readiness proves actual mounting; do not wait on PVC first.
		if !ready {
			action := "create"
			if actual != nil {
				action = "reconcile"
			}
			changes = append(changes, platformapply.Change{Component: step.name, Action: action, Detail: "reconcile explicit observability component"})
		}
		statuses = append(statuses, platformapply.ComponentStatus{Name: step.name, Category: "observability", State: state(ready, "available", "unavailable"), Reference: "ingestion-unverified"})
	}
	return changes, statuses, nil
}
func (k *KubernetesDeployment) preflightStack(ctx context.Context, request platformapply.DeploymentRequest, contextName, namespace string) error {
	config, enabled := stackConfig(request)
	if !enabled {
		return nil
	}
	// Check class and Secret metadata only, never fetch secret values.
	if _, err := k.run(ctx, nil, "--context", contextName, "get", "storageclass", config["storageClass"].(string), "-o", "name"); err != nil {
		return fmt.Errorf("observability storage class unavailable")
	}
	if _, err := k.run(ctx, nil, "--context", contextName, "--namespace", namespace, "get", "secret", config["grafanaAdminSecretRef"].(string), "-o", "name"); err != nil {
		return fmt.Errorf("Grafana credential reference unavailable")
	}
	for _, step := range stackSteps(request, namespace) {
		if !plannedComponent(request.TargetChanges, step.name) {
			continue
		}
		kind := stackKind(step)
		for _, verb := range []string{"get", "create", "patch"} {
			if err := k.requireRBAC(ctx, contextName, verb, kind, namespace); err != nil {
				return err
			}
		}
		if kind == "deployment" {
			if err := k.requireRBAC(ctx, contextName, "watch", "deployments.apps", namespace); err != nil {
				return err
			}
		}
	}
	return nil
}

// Repair complete owned fields; apply alone does not remove drift introduced by another manager.
func (k *KubernetesDeployment) repairStack(ctx context.Context, contextName, namespace string, step manifestStep) error {
	field := ""
	switch stackKind(step) {
	case "deployment", "networkpolicy":
		field = "spec"
	case "service":
		field = "spec/selector"
	case "configmap":
		field = "data"
	default:
		return nil
	}
	value := step.object["spec"]
	if field == "spec/selector" {
		value = step.object["spec"].(map[string]any)["selector"]
	}
	if field == "data" {
		value = step.object["data"]
	}
	patch, err := json.Marshal([]any{map[string]any{"op": "replace", "path": "/" + field, "value": value}})
	if err != nil {
		return err
	}
	_, err = k.run(ctx, nil, "--context", contextName, "--namespace", namespace, "patch", stackKind(step), objectName(step.object), "--type=json", "-p", string(patch))
	return err
}
