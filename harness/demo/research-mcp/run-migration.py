# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
"""Explicit authorized operator runner. Credentials stay in memory/temporary Job."""
import argparse
import base64
import json
import re
import subprocess
import time
import uuid

SECRET = "arn:aws:secretsmanager:ap-southeast-2:931228356546:secret:rds!db-16fd8654-0050-42b0-a6df-12ec85ea7c74-poYtb1"
OWNER = "product-release-20261023"


def command(args, data=None):
    result = subprocess.run(args, input=data, text=True, capture_output=True, timeout=150)
    if result.returncode:
        raise RuntimeError("Operator command failed; raw output suppressed")
    return result.stdout


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--kubeconfig", required=True)
    parser.add_argument("--image", required=True)
    parser.add_argument("--apply", action="store_true")
    parser.add_argument("--role", default="")
    args = parser.parse_args()
    if not re.fullmatch(r"931228356546\.dkr\.ecr\.ap-southeast-2\.amazonaws\.com/agenova-demo/worker@sha256:[a-f0-9]{64}", args.image):
        raise RuntimeError("An exact reviewed operator image digest is required")
    if args.apply and not re.fullmatch(r"agenova_[a-z0-9_]{1,48}", args.role):
        raise RuntimeError("Apply needs the previously inspected exact runtime role")
    identity = json.loads(command(["aws", "sts", "get-caller-identity", "--profile", "agenova-neo", "--output", "json"]))
    if identity.get("Account") != "931228356546":
        raise RuntimeError("Wrong AWS account")
    kube = ["kubectl", "--kubeconfig", args.kubeconfig, "--context", "agenova-neo-release", "-n", "agenova-system"]
    if args.apply:
        deployment = json.loads(command(kube + ["get", "deployment", "agenova-member-control-plane", "-o", "json"]))
        if deployment["spec"]["replicas"] != 0:
            raise RuntimeError("Stop the single writer before applying SQL changes")
        claims = json.loads(command(kube + ["get", "sandboxclaims.extensions.agents.x-k8s.io", "-o", "json"]))
        if claims.get("items"):
            raise RuntimeError("Active SandboxClaims prevent migration")
    name = "agenova-product-migrate-" + uuid.uuid4().hex[:10]
    labels = {"agenova.io/operator-run": OWNER}
    created = []
    try:
        # This one exact Secret was explicitly authorized. Never print/save it.
        response = json.loads(command(["aws", "secretsmanager", "get-secret-value", "--profile", "agenova-neo", "--region", "ap-southeast-2", "--secret-id", SECRET, "--output", "json"]))
        raw = response.get("SecretString", "")
        credentials = json.loads(raw)
        if credentials.get("username") != "agenova_operator" or not credentials.get("password"):
            raise RuntimeError("Unexpected operator credential")
        secret = {"apiVersion": "v1", "kind": "Secret", "metadata": {"name": name, "labels": labels}, "type": "Opaque", "data": {"credentials.json": base64.b64encode(raw.encode()).decode()}}
        command(kube + ["create", "-f", "-"], json.dumps(secret))
        created.append("secret")
        job = {"apiVersion": "batch/v1", "kind": "Job", "metadata": {"name": name, "labels": labels}, "spec": {"backoffLimit": 0, "activeDeadlineSeconds": 120, "ttlSecondsAfterFinished": 3600, "template": {"metadata": {"labels": labels}, "spec": {"restartPolicy": "Never", "automountServiceAccountToken": False, "securityContext": {"runAsNonRoot": True, "runAsUser": 65532, "fsGroup": 65532, "seccompProfile": {"type": "RuntimeDefault"}}, "containers": [{"name": "migrate", "image": args.image, "args": (["-apply", "-role", args.role] if args.apply else []), "resources": {"requests": {"cpu": "20m", "memory": "48Mi"}, "limits": {"cpu": "100m", "memory": "128Mi"}}, "securityContext": {"allowPrivilegeEscalation": False, "readOnlyRootFilesystem": True, "capabilities": {"drop": ["ALL"]}}, "volumeMounts": [{"name": "operator", "mountPath": "/run/operator", "readOnly": True}, {"name": "rds", "mountPath": "/etc/rds", "readOnly": True}]}], "volumes": [{"name": "operator", "secret": {"secretName": name, "defaultMode": 288}}, {"name": "rds", "configMap": {"name": "agenova-member-config", "items": [{"key": "rds-ca.pem", "path": "rds-ca.pem"}]}}]}}}}
        command(kube + ["create", "-f", "-"], json.dumps(job))
        created.append("job")
        # Fail fast on a terminal error rather than keeping the shared writer
        # stopped until a wait-for-success timeout expires.
        deadline = time.monotonic() + 130
        while True:
            status = json.loads(command(kube + ["get", "job", name, "-o", "json"]))
            conditions = {c.get("type") for c in status.get("status", {}).get("conditions", []) if c.get("status") == "True"}
            if "Complete" in conditions:
                break
            if "Failed" in conditions:
                raise RuntimeError("Trusted operator Job failed")
            if time.monotonic() >= deadline:
                raise RuntimeError("Trusted operator Job timed out")
            time.sleep(2)
        logs = command(kube + ["logs", "job/" + name])
        # The trusted image emits only this schema/grant summary; validate it.
        summary = json.loads(logs)
        allowed = {"mode", "schema", "runtimeRoles", "checks", "migrationBaseline", "schemaBefore", "schemaAfter", "runtimeRole", "minimalGrantsVerified", "memberAndProjectCountsUnchanged"}
        if set(summary) - allowed:
            raise RuntimeError("Unexpected operator output; raw logs suppressed")
        print(json.dumps(summary))
    finally:
        cleanup_failed = False
        for kind in reversed(created):
            try:
                owner = command(kube + ["get", kind, name, "-o", "jsonpath={.metadata.labels.agenova\\.io/operator-run}"])
                if owner != OWNER:
                    raise RuntimeError("Temporary object ownership changed")
                command(kube + ["delete", kind, name, "--wait=true", "--timeout=40s"])
            except Exception:
                cleanup_failed = True
        if cleanup_failed:
            raise RuntimeError("Temporary object cleanup incomplete; check only the named owned objects")


if __name__ == "__main__":
    try:
        main()
    except Exception:
        raise SystemExit("Migration runner failed; raw diagnostics and credentials suppressed. Check only owned Job status.") from None
