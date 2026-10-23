# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
"""Build/test the separate Researcher image from allowlisted source; never push."""
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import uuid

ROOT = Path(__file__).resolve().parents[3]


def main():
    output = ROOT / ".tmp/researcher-release"
    context = output / ("context-" + uuid.uuid4().hex[:12])
    context.mkdir(parents=True)
    dockerfile = "harness/demo/research-mcp/worker.Dockerfile"
    files = [ROOT / "go.mod", ROOT / "go.sum", ROOT / dockerfile]
    for directory in ["internal/workerprotocol", "cmd/agenova-demo-worker", "harness/integration/agentsandbox/testworker"]:
        files.extend(p for p in (ROOT / directory).glob("*.go") if not p.name.endswith("_test.go"))
    hashes = {}
    for source in files:
        if source.is_symlink() or not source.is_file():
            raise RuntimeError("Source must be a regular public source file")
        relative = source.relative_to(ROOT)
        target = context / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(source, target)
        hashes[relative.as_posix()] = hashlib.sha256(target.read_bytes()).hexdigest()
    source_hash = hashlib.sha256(json.dumps(hashes, sort_keys=True).encode()).hexdigest()
    tag = "agenova-researcher:local-" + source_hash[:12]
    subprocess.run(["docker", "build", "--platform", "linux/amd64", "-f", str(context / dockerfile), "-t", tag, str(context)], check=True)
    task = {"claimId": "synthetic-image-check", "objective": "Research the meeting features", "modelProfile": "test", "mode": "react"}
    unsupported = {"allowed": True, "text": json.dumps({"action": "finish", "answer": "Unsupported answer"})}
    smoke = subprocess.run(["docker", "run", "--rm", "-i", "--network", "none", "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--entrypoint", "/agenova-demo-worker", tag], input=json.dumps(task) + "\n" + json.dumps(unsupported) + "\n", text=True, capture_output=True, timeout=30)
    messages = [json.loads(line) for line in smoke.stdout.splitlines()]
    if smoke.returncode != 1 or len(messages) != 1 or "Researcher:" not in messages[0].get("operation", {}).get("prompt", "") or any(m.get("result") for m in messages):
        raise RuntimeError("Exact Researcher image did not reject an unsupported answer")
    # The real-cloud defect had a successful but unrelated CRM observation.
    # Test that exact image path, not just a zero-observation scripted refusal.
    crm_task = {**task, "tools": [{"operation": "crm.read", "description": "Read synthetic CRM", "resourceScope": "crm:customer-retention", "parameter": "dataset_id", "allowedValues": ["customer-retention"]}]}
    crm_call = {"allowed": True, "text": json.dumps({"action": "tool", "tool": "crm.read", "resource": "crm:customer-retention", "input": "customer-retention", "answer": ""})}
    observation = {"allowed": True, "text": '{"synthetic":true,"customers":[{"id":"C001"}]}'}
    for answer, expected_code in [("RESEARCH_TASK_BLOCKED", 1), ("Synthetic customer C001 was observed.", 0)]:
        final = {"allowed": True, "text": json.dumps({"action": "finish", "answer": answer})}
        scenario_task = crm_task if expected_code else {**crm_task, "objective": "Read the synthetic customer-retention dataset and cite an observed customer ID."}
        stream = "".join(json.dumps(item) + "\n" for item in [scenario_task, crm_call, observation, final])
        checked = subprocess.run(["docker", "run", "--rm", "-i", "--network", "none", "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--entrypoint", "/agenova-demo-worker", tag], input=stream, text=True, capture_output=True, timeout=30)
        records = [json.loads(line) for line in checked.stdout.splitlines()]
        results = [record["result"] for record in records if "result" in record]
        if checked.returncode != expected_code or results != ([] if expected_code else [answer]) or not any(record.get("operation", {}).get("tool") == "crm.read" for record in records):
            raise RuntimeError("Exact Researcher image completion regression")
    image_id = subprocess.run(["docker", "image", "inspect", tag, "--format", "{{.Id}}"], capture_output=True, text=True, check=True).stdout.strip()
    summary = {"tag": tag, "image": image_id, "source_hash": source_hash, "public_source_hashes": hashes, "unsupported_answer_rejected": True, "blocked_after_observation_rejected": True, "observed_answer_preserved": True, "real_model": False, "pushed": False}
    (output / "image-check.json").write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({k: v for k, v in summary.items() if k != "public_source_hashes"}))


if __name__ == "__main__":
    main()
