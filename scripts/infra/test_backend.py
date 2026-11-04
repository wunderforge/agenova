# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
import copy
import unittest
from check_backend import check, BUCKET, ACCOUNT

class BackendTests(unittest.TestCase):
    def test_valid_and_invalid(self):
        metadata = {'backend': {'type': 's3', 'config': {'bucket': BUCKET, 'key': 'production/eks/terraform.tfstate', 'region': 'ap-southeast-2', 'encrypt': True, 'use_lockfile': True}}}
        identity = {'Account': ACCOUNT, 'Arn': f'arn:aws:iam::{ACCOUNT}:user/agenova-demo-operator'}
        check(metadata, identity, 'production/eks')
        for field, value in [('bucket', 'other'), ('key', 'other'), ('use_lockfile', False), ('encrypt', False)]:
            bad = copy.deepcopy(metadata)
            bad['backend']['config'][field] = value
            with self.assertRaises(ValueError):
                check(bad, identity, 'production/eks')
        for arn in [f'arn:aws:iam::{ACCOUNT}:root', f'arn:aws:iam::{ACCOUNT}:user/other']:
            with self.assertRaises(ValueError):
                check(metadata, {'Account': ACCOUNT, 'Arn': arn}, 'production/eks')
