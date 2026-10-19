# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
"""Disposable local TLS PostgreSQL + actual Go integration suites. No AWS commands."""
import argparse
import json
import os
from pathlib import Path
import secrets
import subprocess
import tempfile
import time
from urllib.parse import urlencode, quote
import uuid

ROOT = Path(__file__).resolve().parents[3]


def run(command, *, env=None, timeout=90, capture=True):
    result = subprocess.run(command, cwd=ROOT, env=env, timeout=timeout,
                            capture_output=capture, text=True)
    if result.returncode:
        # Never dump generated credentials, environment or container logs.
        raise RuntimeError("Local command failed: " + Path(command[0]).name)
    return result.stdout.strip() if capture else ""


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--openssl", required=True)
    p.add_argument("--image", required=True, help="Official postgres image pinned by sha256")
    p.add_argument("--go", default="go")
    p.add_argument("--docker", default="docker")
    args = p.parse_args()
    if not args.image.startswith("postgres@sha256:") or len(args.image.split(":")[-1]) != 64:
        p.error("Use the reviewed immutable postgres image digest")
    container = "agenova-cicd-sql-" + uuid.uuid4().hex[:12]
    output = ROOT / ".tmp/cicd-integration/local-postgres-summary.json"
    output.parent.mkdir(parents=True, exist_ok=True)
    summary = {"scope": "local-only", "postgres_image": args.image,
               "real_google_login": False, "cloud_resources_created": False,
               "tests_passed": False, "container_removed": False}
    with tempfile.TemporaryDirectory(prefix="agenova-cicd-sql-") as directory:
        private = Path(directory)
        # New disposable fixture material only; never read existing private files.
        certdir = private / "tls"
        certdir.mkdir()
        run([args.openssl, "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "2",
             "-sha256", "-subj", "/CN=localhost", "-addext", "subjectAltName=DNS:localhost,IP:127.0.0.1",
             "-keyout", str(certdir / "server.key"), "-out", str(certdir / "server.crt")])
        password = secrets.token_hex(32)
        envfile = private / "postgres-fixture.env"
        envfile.write_text("POSTGRES_USER=review_migration\nPOSTGRES_DB=agenova_review\nPOSTGRES_PASSWORD=" + password + "\n", encoding="utf-8")
        if os.name != "nt":
            envfile.chmod(0o600)
        started = False
        try:
            startup = ("cp /bootstrap-tls/server.* /run/postgres-tls/; "
                       "chown postgres:postgres /run/postgres-tls/server.*; "
                       "chmod 600 /run/postgres-tls/server.key; "
                       "exec docker-entrypoint.sh postgres -c ssl=on "
                       "-c ssl_cert_file=/run/postgres-tls/server.crt "
                       "-c ssl_key_file=/run/postgres-tls/server.key")
            # No generated password on argv; all local ports are loopback only.
            run([args.docker, "run", "-d", "--name", container,
                 "--label", "agenova.local-test=memberlab", "--restart=no",
                 "--publish", "127.0.0.1::5432", "--memory", "384m", "--cpus", "1",
                 "--read-only", "--tmpfs", "/var/lib/postgresql/data:rw,size=512m",
                 "--tmpfs", "/var/run/postgresql:rw", "--tmpfs", "/tmp:rw",
                 "--tmpfs", "/run/postgres-tls:rw", "--env-file", str(envfile),
                 "--mount", f"type=bind,source={certdir},target=/bootstrap-tls,readonly",
                 "--entrypoint", "bash", args.image, "-ceu", startup])
            started = True
            ready = False
            for _ in range(30):
                check = subprocess.run([args.docker, "exec", container, "pg_isready", "-U", "review_migration", "-d", "agenova_review"],
                                       capture_output=True, timeout=10)
                if check.returncode == 0:
                    ready = True
                    break
                time.sleep(1)
            if not ready:
                raise RuntimeError("Disposable PostgreSQL did not become ready")
            port = run([args.docker, "port", container, "5432/tcp"]).rsplit(":", 1)[1]
            query = urlencode({"sslmode": "verify-full", "sslrootcert": str(certdir / "server.crt"), "connect_timeout": "5"})
            dsn = "postgres://review_migration:" + quote(password) + "@localhost:" + port + "/agenova_review?" + query
            env = {**os.environ, "AGENOVA_TEST_POSTGRES": dsn, "AGENOVA_TEST_MEMBERLAB_TLS": "1",
                   "GOCACHE": os.environ.get("GOCACHE", str(ROOT / ".tmp/gocache"))}
            run([args.go, "test", "-count=1", "-timeout=180s", "./internal/authentication/postgres",
                 "./internal/websession", "./internal/console", "./internal/workstore",
                 "./internal/connectedclient", "./cmd/agenova-control-plane"], env=env, timeout=600, capture=False)
            summary["tests_passed"] = True
            summary["tls"] = "verify-full"
        finally:
            # Exact freshly generated name only. No volume/tag sweeps.
            removal = subprocess.run([args.docker, "rm", "-f", container], capture_output=True, timeout=60)
            summary["container_removed"] = removal.returncode == 0
            output.write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")
            if started and not summary["container_removed"]:
                raise RuntimeError("Local test cleanup failed; inspect only container " + container)
    print(json.dumps(summary))


if __name__ == "__main__":
    main()
