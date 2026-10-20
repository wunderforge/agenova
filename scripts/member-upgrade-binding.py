#!/usr/bin/env python3
# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
"""Print non-secret image-upgrade bindings from a reviewed Deployment JSON."""
import hashlib
import json
import sys


def main():
    deployment = json.load(sys.stdin)
    if deployment.get("kind") != "Deployment":
        raise ValueError("expected Deployment JSON")
    spec = deployment["spec"]
    if spec.get("replicas") != 1 or spec.get("strategy", {}).get("type") != "Recreate":
        raise ValueError("expected single-writer Recreate Deployment")
    template = spec["template"]
    containers = [c for c in template["spec"]["containers"] if c["name"] == "control-plane"]
    if len(containers) != 1:
        raise ValueError("expected one named control-plane container")
    image = containers[0].pop("image")
    # Match Go encoding/json's map ordering and HTML/line-separator escaping.
    canonical = json.dumps(template, sort_keys=True, separators=(",", ":"), ensure_ascii=False)
    for character, escaped in (("&", "\\u0026"), ("<", "\\u003c"), (">", "\\u003e"),
                               ("\u2028", "\\u2028"), ("\u2029", "\\u2029")):
        canonical = canonical.replace(character, escaped)
    result = {
        "mode": "member-upgrade",
        "deployment-name": deployment["metadata"]["name"],
        "deployment-uid": deployment["metadata"]["uid"],
        "pod-template-sha256": hashlib.sha256(canonical.encode()).hexdigest(),
        "control-plane-image": image,
    }
    print(json.dumps(result, indent=2))


if __name__ == "__main__":
    try:
        main()
    except (ValueError, KeyError, TypeError):
        sys.exit("Invalid Deployment input; no binding generated")
