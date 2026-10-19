# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
"""Local image checks using actual Dockerfiles and allowlisted public contexts; never pushes."""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import uuid

ROOT = Path(__file__).resolve().parents[3]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--docker", default="docker")
    args = parser.parse_args()
    output = ROOT / ".tmp/cicd-integration/images"
    output.mkdir(parents=True, exist_ok=True)
    result = {"scope": "local-only", "image_pushed": False, "cloud_resources_created": False, "images": {}}
    for component, dockerfile, directories in [
        ("control-plane", "deploy/reference/Dockerfile", ["api", "internal", "cmd/agenova-control-plane"]),
        ("worker", "harness/integration/agentsandbox/testworker/Dockerfile", ["internal/workerprotocol", "cmd/agenova-demo-worker", "harness/integration/agentsandbox/testworker"]),
    ]:
        context = output / (component + "-" + uuid.uuid4().hex[:12])
        context.mkdir()
        files = [ROOT / "go.mod", ROOT / "go.sum", ROOT / dockerfile]
        for directory in directories:
            files.extend(p for p in (ROOT / directory).rglob("*")
                         if p.is_file() and not p.is_symlink()
                         and p.suffix in {".go", ".sql", ".html", ".css", ".js"}
                         and not p.name.endswith("_test.go") and "testdata" not in p.parts)
        hashes = {}
        for path in files:
            relative = path.relative_to(ROOT)
            destination = context / relative
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(path, destination)
            hashes[relative.as_posix()] = hashlib.sha256(destination.read_bytes()).hexdigest()
        tag = "agenova-cicd-local:" + component + "-" + hashlib.sha256(json.dumps(hashes, sort_keys=True).encode()).hexdigest()[:12]
        subprocess.run([args.docker, "build", "--platform", "linux/amd64", "--build-arg", "TARGETARCH=amd64", "--file", str(context / dockerfile), "--tag", tag, str(context)], check=True, cwd=ROOT)
        image_id = subprocess.run([args.docker, "image", "inspect", tag, "--format", "{{.Id}}"], capture_output=True, text=True, check=True).stdout.strip()
        command = [args.docker, "run", "--rm", "--network", "none", "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges"]
        if component == "control-plane":
            command += ["--env", "AGENOVA_AUTH_MODE=member", tag]
            expected = "member configuration is required"
        else:
            command += ["--entrypoint", "/agenova-demo-worker", tag]
            expected = "demo worker task failed"
        smoke = subprocess.run(command, input="", capture_output=True, text=True, timeout=30)
        if smoke.returncode != 1 or expected not in smoke.stderr:
            raise RuntimeError("Local fail-closed executable smoke failed: " + component)
        result["images"][component] = {"tag": tag, "id": image_id, "fail_closed_smoke": True, "source_hashes": hashes}
    (output / "summary.json").write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({k: v for k, v in result.items() if k != "images"} | {"images": {k: {a: b for a, b in v.items() if a != "source_hashes"} for k, v in result["images"].items()}}))


if __name__ == "__main__":
    main()
