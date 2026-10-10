// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package bundled

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/credentials"
	"github.com/wunderforge/agenova/internal/platform"
	"github.com/wunderforge/agenova/internal/platformapply"
)

var (
	installedCredentialContext   = flag.String("kube-context", "", "explicit existing test kubectl context (required)")
	installedCredentialNamespace = flag.String("namespace", "", "new exclusive agenova-155- namespace (required)")
	installedCredentialImage     = flag.String("control-plane-image", "", "preloaded unique task-only agenova-control-plane image (required)")
	installedCredentialImageTag  = regexp.MustCompile(`^agenova-control-plane:(?:155-|e14-213-)[a-z0-9][a-z0-9.-]{7,79}$`)
)

// This opt-in campaign exercises the installed binary's authorized startup
// check using generated reference resources. It neither allocates workers nor
// calls a model provider; provider health remains explicitly not checked.
// Compile without touching a cluster with:
//
// go test -tags integration -run '^$' ./internal/adapters/bundled
//
// After building/loading a unique image into the selected existing kind cluster:
//
// go test -v -count=1 -tags integration -run '^TestInstalledCredentialLive' -timeout 4m ./internal/adapters/bundled -args -kube-context <existing-test-context> -namespace agenova-155-<unique-suffix> -control-plane-image agenova-control-plane:e14-213-<unique-suffix>
func TestInstalledCredentialLiveStartupRBACAndPublicExclusion(t *testing.T) {
	if strings.TrimSpace(*installedCredentialContext) != *installedCredentialContext || *installedCredentialContext == "" || len(*installedCredentialContext) > 256 || strings.ContainsAny(*installedCredentialContext, "\x00\r\n") ||
		!credentialNamespace.MatchString(*installedCredentialNamespace) || !strings.HasPrefix(*installedCredentialNamespace, "agenova-155-") || len(*installedCredentialNamespace) <= len("agenova-155-") ||
		!installedCredentialImageTag.MatchString(*installedCredentialImage) {
		t.Fatal("installed credential integration requires explicit -kube-context, a new agenova-155- -namespace and a unique preloaded task-tagged -control-plane-image")
	}
	if _, err := exec.LookPath("kubectl"); err != nil {
		t.Fatal("installed credential integration requires kubectl")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	token := installedSyntheticMaterial(t)
	defer clear(token)
	request := installedCredentialRequest(t, *installedCredentialContext, *installedCredentialNamespace)
	steps, err := referenceSteps(request, *installedCredentialNamespace, false)
	if err != nil {
		t.Fatal("canonical reference resource generation failed")
	}
	for _, step := range steps {
		if step.object["kind"] == "Deployment" {
			container := step.object["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["containers"].([]any)[0].(map[string]any)
			container["image"] = *installedCredentialImage
		}
		encoded, encodeErr := json.Marshal(step.object)
		if encodeErr != nil || installedContainsMaterial(encoded, token) {
			clear(encoded)
			t.Fatal("generated resource contains synthetic external material")
		}
		clear(encoded)
	}
	f := newInstalledCredentialFixture(t, ctx)
	f.logVersions(t, ctx)
	for _, step := range steps {
		f.apply(t, ctx, step.object)
	}

	missing := f.waitForStartup(t, ctx, "missing-selected-secret", "", false, token)
	f.assertEndpointUnavailable(t, ctx, missing, token)
	f.assertPodExclusion(t, missing, token)
	f.assertNamedRole(t, ctx, true)
	t.Log("missing selected Secret: installed process exits with sanitized resolution failure; Pod is not Ready and readyz is unavailable")

	f.apply(t, ctx, map[string]any{"apiVersion": "v1", "kind": "Secret", "metadata": map[string]any{"name": "model-token", "namespace": f.namespace}, "type": "Opaque", "data": map[string][]byte{"token": token}})
	f.restart(t, ctx, missing)
	ready := f.waitForStartup(t, ctx, "selected-secret-ready", missing.Metadata.UID, true, token)
	f.assertReadyAndPublicStatus(t, ctx, ready, request.Platform.Revision, token)
	f.assertPodExclusion(t, ready, token)
	f.assertConfigMapExclusion(t, ctx, token)
	t.Log("selected Secret create and exact Pod restart: installed process is Ready; public status contains reference metadata and reports provider health not checked")

	// Change only this campaign's generated Role, then restart the observed
	// Pod. Deleting one exact UID avoids an old Ready rolling-update replica
	// masking the new process's authorization failure.
	f.apply(t, ctx, roleObject(f.namespace))
	f.assertNamedRole(t, ctx, false)
	f.restart(t, ctx, ready)
	forbidden := f.waitForStartup(t, ctx, "selected-get-forbidden", ready.Metadata.UID, false, token)
	f.assertEndpointUnavailable(t, ctx, forbidden, token)
	f.assertPodExclusion(t, forbidden, token)
	t.Log("selected Secret get removed from the same owned Role: restarted installed process fails closed and exposes no readiness")

	f.apply(t, ctx, roleObject(f.namespace, "model-token"))
	f.assertNamedRole(t, ctx, true)
	f.restart(t, ctx, forbidden)
	restored := f.waitForStartup(t, ctx, "named-get-restored", forbidden.Metadata.UID, true, token)
	f.assertReadyAndPublicStatus(t, ctx, restored, request.Platform.Revision, token)
	f.assertPodExclusion(t, restored, token)
	f.assertConfigMapExclusion(t, ctx, token)
	t.Logf("exact named Secret get restored: installed process is Ready; image=%s imageID=%s; no worker allocation or model-provider call is claimed", *installedCredentialImage, restored.Status.ContainerStatuses[0].ImageID)
}

// This checks campaign setup without executing any Kubernetes command.
func TestInstalledCredentialCampaignCompositionIsCanonical(t *testing.T) {
	request := installedCredentialRequest(t, "kind-explicit-test", "agenova-155-compile")
	selected, err := referenceCredentialSelection(request.Platform, "agenova-155-compile")
	if err != nil || selected == nil || selected.Name != "model-token" || selected.Key != "token" || platform.VerifyResolvedLock(request.Platform, request.Lock) != nil {
		t.Fatal("installed campaign failed its pure canonical composition check")
	}
	steps, err := referenceSteps(request, "agenova-155-compile", false)
	if err != nil || len(steps) != 8 {
		t.Fatal("installed campaign does not generate the reference installation resources")
	}
}

func installedCredentialRequest(t *testing.T, kubeContext, namespace string) platformapply.DeploymentRequest {
	t.Helper()
	base := credentialRequest()
	runtime := base.Platform.Instances[0]
	runtime.AdapterRef = "runtime-adapter"
	runtime.Config["connection"].(map[string]any)["namespace"] = namespace
	model := base.Platform.Instances[1]
	model.Config["endpoint"] = "https://synthetic-provider.invalid/v1"
	resolver := base.Platform.Instances[2]
	resolver.Config["namespace"] = namespace
	instance := func(value platform.ResolvedInstance) v0.PlatformInstance {
		return v0.PlatformInstance{Name: value.Name, AdapterRef: value.AdapterRef, Config: value.Config, CredentialRef: value.CredentialRef}
	}
	input := &v0.Platform{APIVersion: v0.PlatformAPIVersion, Kind: v0.PlatformKind, Metadata: v0.ObjectMeta{Name: "credential-campaign"}, Spec: v0.PlatformSpec{
		Adapters: []v0.PlatformAdapterRequirement{
			{Name: "deployment-adapter", ID: KubernetesDeploymentID, Version: ReferenceVersion},
			{Name: runtime.AdapterRef, ID: AgentSandboxRuntimeID, Version: ReferenceVersion},
			{Name: model.AdapterRef, ID: OpenAICompatibleModelID, Version: ReferenceVersion},
			{Name: resolver.AdapterRef, ID: KubernetesSecretCredentialID, Version: ReferenceVersion},
		},
		Infrastructure: v0.PlatformInfrastructure{
			Deployment:      &v0.PlatformInstance{Name: "reference-install", AdapterRef: "deployment-adapter", Config: map[string]any{"context": kubeContext, "namespace": namespace}},
			RuntimeBackends: []v0.PlatformInstance{instance(runtime)},
			RuntimeProfiles: []v0.PlatformProfile{{Name: "standard-isolated", BackendRef: runtime.Name, Config: map[string]any{"isolation": "dedicated"}}},
		},
		Services: v0.PlatformServices{
			ModelBackends:       []v0.PlatformInstance{instance(model)},
			ModelProfiles:       []v0.PlatformProfile{{Name: "coding-standard", BackendRef: model.Name, Config: map[string]any{"model": "synthetic-model"}}},
			CredentialResolvers: []v0.PlatformInstance{instance(resolver)},
		},
		InitialPolicyRef: &v0.PlatformPolicyReference{ID: "reference-default-deny", Version: "1"},
	}}
	registry, err := NewRegistry()
	if err != nil {
		t.Fatal("bundled registry construction failed")
	}
	resolved, lock, resolveErr := platform.Resolve(input, registry)
	if resolveErr != nil || platform.VerifyResolvedLock(resolved, lock) != nil {
		t.Fatal("canonical credential Platform resolution or lock verification failed")
	}
	request := platformapply.DeploymentRequest{Platform: resolved, Lock: lock, Config: input.Spec.Infrastructure.Deployment.Config}
	if err := newKubernetesDeployment(nil).ValidateComposition(request); err != nil {
		t.Fatal("generated installed composition is invalid")
	}
	return request
}

type installedCredentialFixture struct {
	kubeContext, namespace, owner, uid string
}

type installedCredentialPod struct {
	Metadata struct {
		Name, Namespace, UID, DeletionTimestamp string
		Labels                                  map[string]string
		OwnerReferences                         []struct{ Kind, Name, UID string }
	}
	Spec struct {
		Containers []struct {
			Name, Image string
			Env         []struct {
				Name, Value string
				ValueFrom   struct{ SecretKeyRef any }
			}
			EnvFrom []struct{ SecretRef any }
		}
		Volumes []struct {
			Name      string
			Secret    any
			Projected struct {
				Sources []struct{ Secret any }
			}
		}
	}
	Status struct {
		Conditions        []struct{ Type, Status string }
		ContainerStatuses []struct {
			Name, ImageID string
			Ready         bool
			State         installedCredentialContainerState
			LastState     installedCredentialContainerState
		}
	}
}

type installedCredentialContainerState struct {
	Terminated *struct{ ExitCode int }
	Waiting    *struct{ Reason string }
}

func newInstalledCredentialFixture(t *testing.T, ctx context.Context) *installedCredentialFixture {
	t.Helper()
	owner := installedSyntheticMaterial(t)
	defer clear(owner)
	f := &installedCredentialFixture{kubeContext: *installedCredentialContext, namespace: *installedCredentialNamespace, owner: string(owner[:32])}
	object := map[string]any{"apiVersion": "v1", "kind": "Namespace", "metadata": map[string]any{"name": f.namespace, "labels": map[string]any{"agenova.io/credential-campaign": f.owner}}}
	data := installedObjectJSON(t, object)
	defer clear(data)
	raw, err := f.command(ctx, data, "create", "-f", "-", "-o", "jsonpath={.metadata.uid}")
	defer clear(raw)
	if err != nil || len(raw) == 0 || len(raw) > 128 {
		t.Fatal("exclusive task namespace creation failed; existing namespaces are never adopted")
	}
	f.uid = string(raw)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
		defer cancel()
		metadata, readErr := f.command(cleanupCtx, nil, "get", "namespace", f.namespace, "-o", "json")
		defer clear(metadata)
		var observed struct {
			Metadata struct {
				UID    string
				Labels map[string]string
			}
		}
		if readErr != nil || json.Unmarshal(metadata, &observed) != nil || observed.Metadata.UID != f.uid || observed.Metadata.Labels["agenova.io/credential-campaign"] != f.owner {
			t.Error("cleanup refused: namespace owner and UID guard could not be confirmed")
			return
		}
		options := installedObjectJSON(t, map[string]any{"apiVersion": "v1", "kind": "DeleteOptions", "preconditions": map[string]any{"uid": f.uid}})
		defer clear(options)
		out, deleteErr := f.command(cleanupCtx, options, "delete", "--raw", "/api/v1/namespaces/"+f.namespace, "-f", "-")
		clear(out)
		if deleteErr != nil {
			t.Error("exact owned namespace UID-precondition cleanup failed")
			return
		}
		out, waitErr := f.command(cleanupCtx, nil, "wait", "--for=delete", "namespace/"+f.namespace, "--timeout=30s")
		clear(out)
		if waitErr != nil {
			t.Error("owned namespace deletion was not confirmed")
			return
		}
		t.Log("exact namespace cleanup confirmed with owner guard and server-side UID precondition")
	})
	t.Logf("explicit existing context=%s namespace=%s uid=%s; generated reference resources and synthetic-only material", f.kubeContext, f.namespace, f.uid)
	return f
}

func (f *installedCredentialFixture) apply(t *testing.T, ctx context.Context, object map[string]any) {
	t.Helper()
	data := installedObjectJSON(t, object)
	defer clear(data)
	out, err := f.command(ctx, data, "--namespace", f.namespace, "apply", "--server-side", "--field-manager=agenova-155-installed-campaign", "-f", "-")
	clear(out)
	if err != nil {
		t.Fatal("task-owned generated resource apply failed (raw backend output suppressed)")
	}
}

func (f *installedCredentialFixture) restart(t *testing.T, ctx context.Context, pod installedCredentialPod) {
	t.Helper()
	if !f.ownedPod(pod) {
		t.Fatal("restart refused an unowned Pod")
	}
	options := installedObjectJSON(t, map[string]any{"apiVersion": "v1", "kind": "DeleteOptions", "preconditions": map[string]any{"uid": pod.Metadata.UID}})
	defer clear(options)
	out, err := f.command(ctx, options, "delete", "--raw", "/api/v1/namespaces/"+f.namespace+"/pods/"+pod.Metadata.Name, "-f", "-")
	clear(out)
	if err != nil {
		t.Fatal("exact task Pod UID-precondition restart failed")
	}
}

func (f *installedCredentialFixture) ownedPod(pod installedCredentialPod) bool {
	return pod.Metadata.Namespace == f.namespace && pod.Metadata.UID != "" && strings.HasPrefix(pod.Metadata.Name, controlPlaneName+"-") &&
		pod.Metadata.Labels["app.kubernetes.io/name"] == controlPlaneName && len(pod.Metadata.OwnerReferences) == 1 &&
		pod.Metadata.OwnerReferences[0].Kind == "ReplicaSet" && strings.HasPrefix(pod.Metadata.OwnerReferences[0].Name, controlPlaneName+"-")
}

func (f *installedCredentialFixture) waitForStartup(t *testing.T, ctx context.Context, phase, previousUID string, ready bool, token []byte) installedCredentialPod {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) && ctx.Err() == nil {
		raw, err := f.command(ctx, nil, "--namespace", f.namespace, "get", "pods", "--selector=app.kubernetes.io/name="+controlPlaneName, "-o", "json")
		var list struct{ Items []installedCredentialPod }
		decodeErr := json.Unmarshal(raw, &list)
		leaked := installedContainsMaterial(raw, token)
		clear(raw)
		if err != nil || decodeErr != nil || leaked {
			t.Fatal("owned Pod observation failed or exposed external material")
		}
		for _, pod := range list.Items {
			if !f.ownedPod(pod) || pod.Metadata.UID == previousUID || pod.Metadata.DeletionTimestamp != "" || len(pod.Status.ContainerStatuses) != 1 {
				continue
			}
			status := pod.Status.ContainerStatuses[0]
			isReady := false
			for _, condition := range pod.Status.Conditions {
				if condition.Type == "Ready" && condition.Status == "True" {
					isReady = true
				}
			}
			if ready && isReady && status.Ready && status.ImageID != "" {
				t.Logf("phase=%s pod=%s uid=%s ready=true imageID=%s", phase, pod.Metadata.Name, pod.Metadata.UID, status.ImageID)
				return pod
			}
			failed := (status.State.Terminated != nil && status.State.Terminated.ExitCode != 0) || (status.LastState.Terminated != nil && status.LastState.Terminated.ExitCode != 0)
			if !ready && failed && !isReady && !status.Ready && f.sanitizedFailureLog(ctx, pod, token) {
				t.Logf("phase=%s pod=%s uid=%s ready=false startup=sanitized-resolution-failure imageID=%s", phase, pod.Metadata.Name, pod.Metadata.UID, status.ImageID)
				return pod
			}
		}
		select {
		case <-ctx.Done():
		case <-time.After(250 * time.Millisecond):
		}
	}
	t.Fatalf("installed startup phase %s did not reach its required bounded Ready/failure observation", phase)
	return installedCredentialPod{}
}

func (f *installedCredentialFixture) sanitizedFailureLog(ctx context.Context, pod installedCredentialPod, token []byte) bool {
	for _, previous := range []bool{false, true} {
		args := []string{"--namespace", f.namespace, "logs", pod.Metadata.Name, "--container=control-plane", "--tail=20"}
		if previous {
			args = append(args, "--previous")
		}
		logs, err := f.command(ctx, nil, args...)
		acceptable := err == nil && bytes.Contains(logs, []byte(credentials.ErrUnavailable.Error())) && !installedContainsMaterial(logs, token) &&
			!bytes.Contains(logs, []byte("Error from server")) && !bytes.Contains(logs, []byte("forbidden")) && !bytes.Contains(logs, []byte("model-token"))
		clear(logs)
		if acceptable {
			return true
		}
	}
	return false
}

func (f *installedCredentialFixture) assertEndpointUnavailable(t *testing.T, ctx context.Context, pod installedCredentialPod, token []byte) {
	t.Helper()
	out, err := f.command(ctx, nil, "get", "--raw", f.podPath(pod, "readyz"))
	leaked := installedContainsMaterial(out, token)
	clear(out)
	if err == nil || leaked {
		t.Fatal("failed installed startup served readiness or leaked material")
	}
}

func (f *installedCredentialFixture) assertReadyAndPublicStatus(t *testing.T, ctx context.Context, pod installedCredentialPod, revision string, token []byte) {
	t.Helper()
	out, err := f.command(ctx, nil, "get", "--raw", f.podPath(pod, "readyz"))
	ready := bytes.Equal(bytes.TrimSpace(out), []byte("ready"))
	clear(out)
	if err != nil || !ready {
		t.Fatal("Ready installed process did not serve exact readyz response")
	}
	out, err = f.command(ctx, nil, "get", "--raw", f.podPath(pod, "v1/status"))
	var status struct{ Platform, Revision, State, ReadinessScope, ProviderHealth string }
	decodeErr := json.Unmarshal(out, &status)
	leaked := installedContainsMaterial(out, token)
	clear(out)
	if err != nil || decodeErr != nil || leaked || status.Platform != "credential-campaign" || status.Revision != revision || status.State != "installation-ready" || status.ReadinessScope != "installation-components" || status.ProviderHealth != "not-checked" {
		t.Fatal("public installed status differs from secret-free reference readiness scope")
	}
}

func (f *installedCredentialFixture) podPath(pod installedCredentialPod, path string) string {
	return "/api/v1/namespaces/" + f.namespace + "/pods/" + pod.Metadata.Name + ":8080/proxy/" + path
}

func (f *installedCredentialFixture) assertPodExclusion(t *testing.T, pod installedCredentialPod, token []byte) {
	t.Helper()
	encoded, err := json.Marshal(pod)
	defer clear(encoded)
	if err != nil || installedContainsMaterial(encoded, token) || len(pod.Spec.Containers) != 1 || pod.Spec.Containers[0].Image != *installedCredentialImage {
		t.Fatal("installed Pod contains external material or uses a different image")
	}
	for _, env := range pod.Spec.Containers[0].Env {
		if env.ValueFrom.SecretKeyRef != nil || strings.Contains(strings.ToLower(env.Name), "token") || strings.Contains(strings.ToLower(env.Name), "credential") {
			t.Fatal("external credential entered the installed Pod environment")
		}
	}
	for _, env := range pod.Spec.Containers[0].EnvFrom {
		if env.SecretRef != nil {
			t.Fatal("external Secret entered installed Pod envFrom")
		}
	}
	for _, volume := range pod.Spec.Volumes {
		if volume.Secret != nil {
			t.Fatal("external Secret volume entered installed Pod")
		}
		for _, source := range volume.Projected.Sources {
			if source.Secret != nil {
				t.Fatal("external Secret projection entered installed Pod")
			}
		}
	}
}

func (f *installedCredentialFixture) assertConfigMapExclusion(t *testing.T, ctx context.Context, token []byte) {
	t.Helper()
	out, err := f.command(ctx, nil, "--namespace", f.namespace, "get", "configmap", platformRecord, "-o", "json")
	defer clear(out)
	if err != nil || installedContainsMaterial(out, token) || !bytes.Contains(out, []byte("model-token")) {
		t.Fatal("effective Platform ConfigMap lost reference metadata or gained external material")
	}
}

func (f *installedCredentialFixture) assertNamedRole(t *testing.T, ctx context.Context, namedGet bool) {
	t.Helper()
	identity := "system:serviceaccount:" + f.namespace + ":" + controlPlaneName
	for _, operation := range []struct {
		verb, resource string
		allowed        bool
	}{{"get", "secrets/model-token", namedGet}, {"get", "secrets/unrelated", false}, {"list", "secrets", false}, {"watch", "secrets", false}} {
		out, err := f.command(ctx, nil, "--namespace", f.namespace, "--as", identity, "auth", "can-i", operation.verb, operation.resource)
		answer := strings.TrimSpace(string(out))
		clear(out)
		if (operation.allowed && (err != nil || answer != "yes")) || (!operation.allowed && (err == nil || answer != "no")) {
			t.Fatal("installed service account effective Secret RBAC exceeds or lacks exact named get")
		}
	}
}

func (f *installedCredentialFixture) logVersions(t *testing.T, ctx context.Context) {
	t.Helper()
	out, err := f.command(ctx, nil, "version", "-o", "json")
	defer clear(out)
	var version struct {
		ClientVersion struct{ GitVersion string }
		ServerVersion struct{ GitVersion string }
	}
	if err != nil || json.Unmarshal(out, &version) != nil || version.ClientVersion.GitVersion == "" || version.ServerVersion.GitVersion == "" {
		t.Fatal("explicit test-cluster version observation failed")
	}
	t.Logf("kubectl=%s server=%s explicit-image=%s", version.ClientVersion.GitVersion, version.ServerVersion.GitVersion, *installedCredentialImage)
}

const installedCredentialMaxOutput = 2 << 20

type installedCredentialBuffer struct{ data bytes.Buffer }

func (b *installedCredentialBuffer) Write(value []byte) (int, error) {
	if len(value) > installedCredentialMaxOutput-b.data.Len() {
		return 0, credentials.ErrUnavailable
	}
	return b.data.Write(value)
}

func (f *installedCredentialFixture) command(ctx context.Context, input []byte, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "kubectl", append([]string{"--context", f.kubeContext, "--request-timeout=5s"}, args...)...)
	cmd.WaitDelay = time.Second
	cmd.Stdin = bytes.NewReader(input)
	var stdout installedCredentialBuffer
	defer clear(stdout.data.Bytes())
	cmd.Stdout, cmd.Stderr = &stdout, io.Discard
	if cmd.Run() != nil {
		return bytes.Clone(stdout.data.Bytes()), credentials.ErrUnavailable
	}
	return bytes.Clone(stdout.data.Bytes()), nil
}

func installedObjectJSON(t *testing.T, object map[string]any) []byte {
	t.Helper()
	data, err := json.Marshal(object)
	if err != nil {
		t.Fatal("task-owned resource encoding failed")
	}
	return data
}

func installedSyntheticMaterial(t *testing.T) []byte {
	t.Helper()
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		t.Fatal("synthetic material generation failed")
	}
	encoded := make([]byte, hex.EncodedLen(len(value)))
	hex.Encode(encoded, value)
	clear(value)
	return encoded
}

func installedContainsMaterial(data, token []byte) bool {
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(token)))
	defer clear(encoded)
	base64.StdEncoding.Encode(encoded, token)
	return bytes.Contains(data, token) || bytes.Contains(data, encoded)
}
