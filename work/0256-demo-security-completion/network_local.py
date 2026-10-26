# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
"""Local-only real Pod network/protocol evidence. Model replies are deterministic fixtures.

This does not claim a real-model or AWS acceptance. No credentials are read.
"""
import json
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / ".tmp/demo-256"
K = ["kubectl", "--kubeconfig", str(OUT / "kind.kubeconfig"), "--context", "kind-agenova-256-network", "-n", "agenova-256-network"]


def run(args, input=None, timeout=45):
    result = subprocess.run(args, input=input, text=True, encoding="utf-8", capture_output=True, timeout=timeout)
    if result.returncode:
        raise RuntimeError("Local diagnostic command failed: " + result.stderr[-1500:])
    return result.stdout


def probe(pod):
    return json.loads(run(K + ["exec", pod, "-c", "agent", "--", "/agenova-demo-worker", "--native-probe"]))


def main():
    identities = {name: json.loads(run(K + ["get", "pod", name, "-o", "json"])) for name in ["native-control", "native-denied"]}
    control, denied = probe("native-control"), probe("native-denied")
    # Drive the actual image's local action with a deterministic model-response fixture.
    native = {"action": "native_https", "tool": "", "resource": "", "input": "https://jitsi.org/security/", "answer": "", "completion": "", "reason": "", "target": ""}
    finish = {"action": "finish", "tool": "", "resource": "", "input": "", "answer": "Direct HTTPS timed out; the page could not be retrieved.", "completion": "incomplete", "reason": "network-unavailable", "target": "direct-https"}
    lines = [{"claimId": "local-fixture-session", "modelProfile": "fixture", "mode": "react", "objective": "Read https://jitsi.org/security/; try direct HTTPS if the web tool is unavailable."}, {"allowed": True, "text": json.dumps(native)}, {"allowed": True}, {"allowed": True}, {"allowed": True, "text": json.dumps(finish)}]
    output = run(K + ["exec", "-i", "native-denied", "-c", "agent", "--", "/agenova-demo-worker"], "\n".join(json.dumps(row) for row in lines) + "\n")
    messages = [json.loads(row) for row in output.splitlines()]
    reports = [row["operation"] for row in messages if row.get("operation", {}).get("kind", "").startswith("native-")]
    checks = {
        "sameImage": identities["native-control"]["status"]["containerStatuses"][0]["imageID"] == identities["native-denied"]["status"]["containerStatuses"][0]["imageID"],
        "sameNode": identities["native-control"]["spec"]["nodeName"] == identities["native-denied"]["spec"]["nodeName"],
        "samePublicIP": control.get("ip") == denied.get("ip"),
        "controlHTTPSReceived": control.get("phase") == "http" and control.get("result") == "received" and 200 <= control.get("statusCode", 0) < 300,
        "deniedTCPTimeout": denied.get("phase") == "tcp" and denied.get("result") == "timeout",
        "actualImageReports": len(reports) == 2 and reports[0]["kind"] == "native-start" and reports[1].get("native", {}).get("phase") == "tcp" and reports[1]["native"]["result"] == "timeout",
        "incompleteOutputRetained": messages[-1].get("result") == finish["answer"],
        "sameDeniedPod": json.loads(run(K + ["get", "pod", "native-denied", "-o", "json"]))["metadata"]["uid"] == identities["native-denied"]["metadata"]["uid"],
    }
    record = {"environment": "isolated local kind + Calico 3.33.0", "model": "deterministic response fixture, NOT real model", "checks": checks, "control": control, "denied": denied, "reports": reports, "pods": {name: {"uid": obj["metadata"]["uid"], "node": obj["spec"]["nodeName"], "imageID": obj["status"]["containerStatuses"][0]["imageID"], "ip": obj["status"]["podIP"]} for name, obj in identities.items()}}
    OUT.mkdir(exist_ok=True)
    (OUT / "local-network-evidence.json").write_text(json.dumps(record, indent=2), encoding="utf-8")
    print(json.dumps(record, indent=2))
    if not all(checks.values()):
        raise SystemExit("Local native-network acceptance failed; retain evidence and investigate")


if __name__ == "__main__":
    main()
