#!/usr/bin/env python3
# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
"""Fail closed unless this checkout uses the expected shared state and AWS account."""
import argparse
import json
import pathlib
import subprocess

BUCKET = 'agenova-tfstate-931228356546-ap-southeast-2'
ACCOUNT = '931228356546'
ROOTS = ('production/eks', 'production/edge', 'member-edge', 'database', 'domain')

def check(metadata, identity, root):
    if root not in ROOTS:
        raise ValueError('Unknown root')
    backend = metadata['backend']
    config = backend['config']
    if (backend['type'] != 's3' or config['bucket'] != BUCKET
            or config['key'] != root + '/terraform.tfstate'
            or config['region'] != 'ap-southeast-2'
            or config.get('use_lockfile') is not True
            or config.get('encrypt') is not True):
        raise ValueError('Unexpected or unlocked backend; run terraform init')
    arn = identity['Arn']
    if identity['Account'] != ACCOUNT or arn.endswith(':root'):
        raise ValueError('Wrong account or forbidden root identity')
    if not (arn == f'arn:aws:iam::{ACCOUNT}:user/agenova-demo-operator'
            or arn.startswith(f'arn:aws:sts::{ACCOUNT}:assumed-role/AWSReservedSSO_AgenovaAdministrator_')):
        raise ValueError('Unrecognized operator identity')

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', required=True, choices=ROOTS)
    args = parser.parse_args()
    try:
        repo = pathlib.Path(__file__).resolve().parents[2]
        metadata = json.loads((repo / 'infra' / args.root / '.terraform/terraform.tfstate').read_text())
        identity = json.loads(subprocess.check_output(['aws', 'sts', 'get-caller-identity', '--output', 'json']))
        check(metadata, identity, args.root)
        print('Shared backend and operator identity verified')
    except (ValueError, KeyError, OSError, subprocess.CalledProcessError) as error:
        parser.exit(1, f'REJECTED: {error}\n')
