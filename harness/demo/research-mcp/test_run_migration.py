# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
"""Fault-injected operator cleanup tests. No real credentials, cluster or AWS."""
import importlib.util
import json
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("migration_runner", Path(__file__).with_name("run-migration.py"))
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)
IMAGE = "931228356546.dkr.ecr.ap-southeast-2.amazonaws.com/agenova-demo/worker@sha256:" + "a" * 64


class CleanupTests(unittest.TestCase):
    def scenario(self, ambiguous="secret", exists=True, wrong_owner=False):
        resources, deleted = {}, []

        def command(args, data=None):
            if args[:2] == ["aws", "sts"]:
                return json.dumps({"Account": "931228356546"})
            if args[:2] == ["aws", "secretsmanager"]:
                return json.dumps({"SecretString": json.dumps({"username": "agenova_operator", "password": "synthetic-test-only"})})
            if "create" in args:
                obj = json.loads(data)
                kind = obj["kind"].lower()
                name = obj["metadata"]["name"]
                if exists or kind != ambiguous:
                    resources[kind] = (name, "foreign" if wrong_owner else runner.OWNER)
                if kind == ambiguous:
                    raise TimeoutError("synthetic ambiguous create")
                return "created"
            if "get" in args:
                kind = args[args.index("get") + 1]
                if kind not in resources:
                    return ""  # Confirmed NotFound only.
                name, owner = resources[kind]
                self.assertIn("jsonpath=", args[-1])
                return name + "\n" + owner
            if "delete" in args:
                kind = args[args.index("delete") + 1]
                deleted.append(kind)
                del resources[kind]
                return "deleted"
            self.fail("Unexpected command")

        argv = ["run-migration.py", "--kubeconfig", "synthetic", "--image", IMAGE]
        with patch.object(runner, "command", side_effect=command), patch("sys.argv", argv):
            with self.assertRaises((TimeoutError, RuntimeError)):
                runner.main()
        return resources, deleted

    def test_secret_created_but_response_lost_is_removed(self):
        resources, deleted = self.scenario()
        self.assertEqual(resources, {})
        self.assertEqual(deleted, ["secret"])

    def test_job_created_but_response_lost_removes_both(self):
        resources, deleted = self.scenario("job")
        self.assertEqual(resources, {})
        self.assertEqual(deleted, ["job", "secret"])

    def test_confirmed_not_found_is_safe(self):
        resources, deleted = self.scenario(exists=False)
        self.assertEqual(resources, {})
        self.assertEqual(deleted, [])

    def test_wrong_owner_is_not_deleted(self):
        resources, deleted = self.scenario(wrong_owner=True)
        self.assertIn("secret", resources)
        self.assertEqual(deleted, [])


if __name__ == "__main__":
    unittest.main()
