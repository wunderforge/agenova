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
    image_id = subprocess.run(["docker", "image", "inspect", tag, "--format", "{{.Id}}"], capture_output=True, text=True, check=True).stdout.strip()
    summary = {"tag": tag, "image": image_id, "source_hash": source_hash, "public_source_hashes": hashes, "unsupported_answer_rejected": True, "real_model": False, "pushed": False}
    (output / "image-check.json").write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({k: v for k, v in summary.items() if k != "public_source_hashes"}))


if __name__ == "__main__":
    main()
