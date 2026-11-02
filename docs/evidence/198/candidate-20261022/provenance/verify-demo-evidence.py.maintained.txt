#!/usr/bin/env python3
# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
"""Offline checks for the retained 2026-10-22 candidate evidence, not acceptance."""
import argparse
import hashlib
import json
from pathlib import Path
import re

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('directory', type=Path)
args = parser.parse_args()
root = args.directory.resolve()

def read(path):
    return json.loads(path.read_text())

def check_rows(directory):
    rows = read(directory / 'summary.json')
    for row in rows:
        scene = row['scenario']
        request = read(directory / (scene + '.request.json'))
        assert request['metadata']['name'] == row['requestRef']
        assert request['spec']['requestedAccess'] == row['requestedAccess']
        if row['showExit'] == 0:
            view = read(directory / (scene + '.show.json'))
        else:
            assert 'HTTP 404' in (directory / (scene + '.show.stderr')).read_text()
            view = {}
        facts = view.get('facts', [])
        state = view.get('state', {})
        assert state.get('effectiveAuthority') == row['effectiveAuthority']
        for operation, counter in [('model.invoke', 'model'), ('tool.invoke', 'mcp')]:
            attempts = sum(f.get('kind') == 'ProviderAttempt' and f.get('operation') == operation for f in facts)
            assert attempts == row['providerAttempts'][operation] == row['delta'][counter]
            assert row['countsAfter'][counter] - row['countsBefore'][counter] == attempts
        for resource in ['pods', 'sandboxclaims']:
            events = [json.loads(line) for line in (directory / (scene + '.' + resource + '.watch.jsonstream')).read_text().splitlines() if line.strip()]
            expected = {kind: sum(e.get('type') == kind for e in events) for kind in ['ADDED', 'MODIFIED', 'DELETED', 'ERROR']}
            assert expected == row['watchEvents'][resource] and expected['ERROR'] == 0
        receipts = [json.loads(line) for line in (directory / (scene + '.mcp.log')).read_text().splitlines() if json.loads(line).get('event') == 'tool']
        new = receipts[row['countsBefore']['mcp']:row['countsAfter']['mcp']]
        assert new == row['mcpToolReceipts']
        allow_ids = [f.get('invocationId') for f in facts if f.get('kind') == 'ToolDecision' and f.get('result') == 'Allow']
        assert sorted(allow_ids) == sorted(r.get('correlation') for r in new)
        if not state.get('claim'):
            assert all(sum(counts.values()) == 0 for counts in row['watchEvents'].values())
    return {row['scenario']: row for row in rows}

# Resolve hash paths under the archive, never follow traversal or symlinks.
manifest = read(root / 'sha256.json')
for name, digest in manifest.items():
    file = root / name
    assert not file.is_symlink() and file.resolve().is_relative_to(root)
    assert hashlib.sha256(file.read_bytes()).hexdigest() == digest, name
actual_files = {str(f.relative_to(root)) for f in root.rglob('*') if f.is_file() and f != root / 'sha256.json'}
assert actual_files == set(manifest), 'hash manifest must cover every retained file'

raw = root / 'raw'
g1 = check_rows(raw / 'group1')
assert g1['a']['principal'] == g1['b']['principal']
assert g1['a']['policyRef'] == g1['b']['policyRef']
assert g1['a']['effectiveAuthority']['resourceScopes'] == ['repo:e15/work-a']
assert g1['b']['effectiveAuthority']['resourceScopes'] == ['repo:e15/work-b']
assert g1['foreign']['phase'] == 'Failed' and g1['foreign']['delta']['mcp'] == 0
assert not [d for d in g1['foreign']['decisions'] if d['kind'] == 'ToolDecision']
assert read(raw / 'group1/b.before-foreign.json')['facts'] == read(raw / 'group1/b.after-foreign.json')['facts']
for scene in ['induced-1', 'induced-2']:
    assert {r['file'] for r in g1[scene]['mcpToolReceipts']} <= {'README.md', 'src/retry.txt'}

# Preserve the failed first attempt rather than hide it.
initial = check_rows(raw / 'group2/installed')
assert initial['succeeded']['phase'] == 'Failed'
g2 = check_rows(raw / 'group2/installed-v2')
for scene, phase in [('succeeded', 'Succeeded'), ('failed', 'Failed'), ('expired', 'Expired')]:
    assert g2[scene]['phase'] == phase
    quiet = read(raw / ('group2/installed-v2/' + scene + '.quiet-window.json'))
    assert quiet['modelDelta'] == quiet['mcpDelta'] == 0
    assert quiet['factsUnchangedDuringQuietWindow']
    before = read(raw / ('group2/installed-v2/' + scene + '.terminal-before.json'))
    after = read(raw / ('group2/installed-v2/' + scene + '.terminal-after.json'))
    assert before['facts'] == after['facts']
    assert {'NetworkRevoked', 'TerminateSucceeded', 'CleanupSucceeded'} <= {f.get('operation') for f in after['facts']}
log = (raw / 'group2/component-terminal-race.log').read_text()
for phase in ['Succeeded', 'Failed', 'Expired']:
    assert 'phase=' + phase + ' lateModel=refused lateTool=refused providerDelta=0 decisionFactDelta=0' in log

g3 = check_rows(raw / 'group3')
extra = check_rows(raw / 'group3-tool-only')
for scene in ['project-denied', 'scope-outside', 'policy-cap']:
    assert g3[scene]['delta'] == {'model': 0, 'mcp': 0}
assert extra['tool-only']['showExit'] != 0 and extra['tool-only']['delta'] == {'model': 0, 'mcp': 0}
assert g3['tool-outside']['phase'] == 'Succeeded'
assert g3['tool-outside']['effectiveAuthority']['tools'] == ['repo.read']
assert 'repo.write' in g3['tool-outside']['requestedAccess']['tools']
assert read(raw / 'group3/project-denied.show.json')['state']['decision']['result'] == 'Deny'

for group in ['group1', 'group2', 'group3']:
    check_rows(root / 'driver-validation' / group)

# Key/pattern check supplements allowlisted copying; not a universal secret scanner.
credential_keys = {'workloadtoken', 'authorization', 'password', 'access_token', 'refresh_token', 'client_secret'}
def inspect(value):
    if isinstance(value, dict):
        for key, child in value.items():
            assert key.lower() not in credential_keys, 'credential value must not be archived'
            inspect(child)
    elif isinstance(value, list):
        for child in value:
            inspect(child)
for file in root.rglob('*'):
    if not file.is_file():
        continue
    text = file.read_text()
    assert not re.search(r'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----|eyJhbGciOi[A-Za-z0-9_-]+\.', text), file
    if file.suffix == '.json':
        try:
            inspect(json.loads(text))
        except json.JSONDecodeError:
            assert not text.strip(), file
print('Verified hashes, raw CLI/facts/watch/receipt consistency and named partial-result assertions; no acceptance inferred.')
