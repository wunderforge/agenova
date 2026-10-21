#!/usr/bin/env python3
# Copyright 2026 Dapeng Zhang and Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
"""Bounded installation-specific TCP checks; never requests provider or metadata credentials."""
import argparse
import datetime
import json
import os
import socket
import subprocess
import uuid

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--context', required=True)
parser.add_argument('--worker', required=True, help='Actual ready Sandbox-owned worker Pod')
parser.add_argument('--allowed-worker', required=True,
                    help='Different original Pod of a live approved public-web Work; never reuse the offline worker')
args = parser.parse_args()
if args.context != 'agenova-demo' or not os.environ.get('KUBECONFIG'):
    raise SystemExit('Explicit agenova-demo context and dedicated KUBECONFIG required')
identity = json.loads(subprocess.check_output(['aws', 'sts', 'get-caller-identity'], timeout=30))
if identity['Account'] != '931228356546' or not identity['Arn'].endswith(':user/agenova-demo-operator'):
    raise SystemExit('Unexpected AWS identity')
expected_endpoint = subprocess.check_output(
    ['aws', 'eks', 'describe-cluster', '--name', 'agenova-demo', '--region', 'ap-southeast-2',
     '--query', 'cluster.endpoint', '--output', 'text'], text=True, timeout=30).strip()
actual_endpoint = subprocess.check_output(
    ['kubectl', '--context', args.context, 'config', 'view', '--minify',
     '-o', 'jsonpath={.clusters[0].cluster.server}'], text=True, timeout=10).strip()
if actual_endpoint != expected_endpoint:
    raise SystemExit('Kubeconfig does not point to the verified EKS cluster')


def kube(*command, namespace='agenova-system', payload=None, check=True):
    result = subprocess.run(['kubectl', '--context', args.context, '--request-timeout=20s',
                           '-n', namespace, *command], input=payload, text=True,
                          capture_output=True, timeout=150)
    if check and result.returncode:
        raise RuntimeError(result.stderr.strip()[:1500])
    return result


def read(*command, **kw):
    return json.loads(kube(*command, '-o', 'json', **kw).stdout)


worker = read('get', 'pod', args.worker)
spec = worker['spec']
assert any(x['kind'] == 'Sandbox' for x in worker['metadata'].get('ownerReferences', [])), 'Not a Sandbox worker'
assert 'agents.x-k8s.io/sandbox-name-hash' in worker['metadata']['labels'], 'Worker policy selector absent'
assert 'agenova.io/network-incarnation' not in worker['metadata']['labels'], 'Use an ungranted worker for this offline check'
assert spec.get('automountServiceAccountToken') is False
assert spec.get('serviceAccountName') == 'default'
assert not spec.get('hostNetwork', False)
assert any(c['type'] == 'Ready' and c['status'] == 'True' for c in worker['status']['conditions'])
for container in spec['containers']:
    assert not container.get('env') and not container.get('envFrom'), 'Unexpected worker environment injection'
    assert not container.get('volumeMounts'), 'Unexpected worker mount'
image = spec['containers'][0]['image']
assert image.startswith('931228356546.dkr.ecr.ap-southeast-2.amazonaws.com/agenova-demo/worker@sha256:')
assert args.allowed_worker != args.worker, 'Offline and delegated workers must be different Pods'
delegated = read('get', 'pod', args.allowed_worker)
assert any(x['kind'] == 'Sandbox' for x in delegated['metadata'].get('ownerReferences', []))
assert delegated['status']['phase'] == 'Running'
delegated_labels = delegated['metadata']['labels']
assert delegated_labels.get('agenova.io/network-incarnation'), 'Approved original-Pod stamp absent'
assert delegated_labels.get('agents.x-k8s.io/claim-uid'), 'Claim binding absent'
assert delegated['spec'].get('automountServiceAccountToken') is False
assert not delegated['spec'].get('hostNetwork', False)
for container in delegated['spec']['containers']:
    assert not container.get('env') and not container.get('envFrom'), 'Unexpected delegated worker environment injection'
    assert not container.get('volumeMounts'), 'Unexpected delegated worker mount'
policies = read('get', 'networkpolicies')['items']
matching_grants = [p for p in policies
                   if p['metadata'].get('labels', {}).get('app.kubernetes.io/managed-by') == 'agenova'
                   and p['spec'].get('podSelector', {}).get('matchLabels') == {
                       'agents.x-k8s.io/claim-uid': delegated_labels['agents.x-k8s.io/claim-uid'],
                       'agenova.io/network-incarnation': delegated_labels['agenova.io/network-incarnation']}]
assert len(matching_grants) == 1, 'Exact original-Pod Work grant missing or ambiguous'
grant = matching_grants[0]
assert any(o['kind'] == 'SandboxClaim' and o['uid'] == delegated_labels['agents.x-k8s.io/claim-uid']
           for o in grant['metadata'].get('ownerReferences', [])), 'Unowned Work grant'
for policy_item in policies:
    if not policy_item['spec'].get('egress') or policy_item is grant:
        continue
    assert policy_item['spec'].get('podSelector') == {'matchExpressions': [
        {'key': 'agents.x-k8s.io/sandbox-name-hash', 'operator': 'DoesNotExist'}]}, \
        'Unexpected additive worker allow; stop and review before delegation'
daemon = read('get', 'daemonset', 'aws-node', namespace='kube-system')
containers = daemon['spec']['template']['spec']['containers']
cni = next(c for c in containers if c['name'] == 'aws-node')
assert any(e['name'] == 'NETWORK_POLICY_ENFORCING_MODE' and e.get('value') == 'strict' for e in cni['env'])
assert any('--enable-network-policy=true' in c.get('args', []) for c in containers)
assert daemon['status'].get('numberReady') == daemon['status']['desiredNumberScheduled']
policy = read('get', 'networkpolicy', 'demo-sandbox-deny-network')['spec']
assert policy.get('egress', []) == [] and policy.get('ingress', []) == []
assert sorted(policy['policyTypes']) == ['Egress', 'Ingress']
admin = read('get', 'clusternetworkpolicy', 'agenova-demo-worker-egress-deny')['spec']
assert admin['tier'] == 'Admin' and admin['priority'] == 10
assert admin['subject'] == {'pods': {
    'namespaceSelector': {'matchLabels': {'kubernetes.io/metadata.name': 'agenova-system'}},
    'podSelector': {'matchExpressions': [
        {'key': 'agents.x-k8s.io/sandbox-name-hash', 'operator': 'Exists'},
        {'key': 'agenova.io/network-incarnation', 'operator': 'DoesNotExist'}]}}}
assert admin['egress'] == [{'name': 'no-direct-network-egress', 'action': 'Deny',
                            'to': [{'networks': ['0.0.0.0/0']}]}]
name = 'e13-network-canary-' + uuid.uuid4().hex[:8]
manifest = {'apiVersion': 'apps/v1', 'kind': 'Deployment', 'metadata': {'name': name},
            'spec': {'replicas': 1, 'selector': {'matchLabels': {'app': name}},
                     'template': {'metadata': {'labels': {'app': name}}, 'spec': {
                         'automountServiceAccountToken': False,
                         'containers': [{'name': 'canary', 'image': image,
                                         'command': ['/bin/sh', '-c', 'exec nc -lk -p 18080 -e cat'],
                                         'securityContext': {'allowPrivilegeEscalation': False,
                                                             'capabilities': {'drop': ['ALL']}},
                                         'readinessProbe': {'tcpSocket': {'port': 18080}}}]}}}}
created = False
results = []
try:
    kube('create', '--validate=false', '-f', '-', payload=json.dumps(manifest))
    created = True
    kube('rollout', 'status', 'deployment/' + name, '--timeout=120s')
    canary = read('get', 'pods', '-l', 'app=' + name)['items'][0]
    # Resolve outside the denied worker so a DNS failure cannot masquerade as a TCP block.
    bedrock_ip = socket.getaddrinfo('bedrock-runtime.ap-southeast-2.amazonaws.com', 443,
                                  socket.AF_INET, socket.SOCK_STREAM)[0][4][0]
    dns_ip = read('get', 'service', 'kube-dns', namespace='kube-system')['spec']['clusterIP']
    for target, address, port in [('bedrock', bedrock_ip, 443),
                                  ('cluster-dns-tcp', dns_ip, 53),
                                  ('synthetic-canary', canary['status']['podIP'], 18080)]:
        for role, pod, allowed in [('trusted-control', canary['metadata']['name'], True),
                                   ('offline-worker', args.worker, False),
                                   ('approved-worker', args.allowed_worker, target != 'synthetic-canary')]:
            original = delegated if role == 'approved-worker' else worker if role == 'offline-worker' else canary
            before = read('get', 'pod', pod)
            assert before['metadata']['uid'] == original['metadata']['uid'], 'Source Pod replaced'
            if role == 'approved-worker':
                assert before['metadata']['labels'].get('agenova.io/network-incarnation') == delegated_labels['agenova.io/network-incarnation']
                assert read('get', 'networkpolicy', grant['metadata']['name'])['metadata']['uid'] == grant['metadata']['uid']
            result = kube('exec', pod, '--', 'nc', '-z', '-w', '3', address, str(port), check=False)
            assert read('get', 'pod', pod)['metadata']['uid'] == original['metadata']['uid'], 'Source Pod changed during probe'
            # exec/transport errors do not count as network denial. kubectl propagates nc exit 1.
            if allowed:
                assert result.returncode == 0, f'Positive control failed for {target}: exit={result.returncode}, {result.stderr[:300]}'
            else:
                assert result.returncode == 1 and 'command terminated with exit code 1' in result.stderr, \
                    f'Expected actual worker TCP denial for {target}, got exit {result.returncode}'
            results.append({'source': role, 'target': target, 'address': address, 'port': port,
                            'connected': result.returncode == 0})
    print(json.dumps({'time': datetime.datetime.now(datetime.timezone.utc).isoformat(),
                      'worker': args.worker, 'workerUID': worker['metadata']['uid'],
                      'cniImage': cni['image'], 'enforcement': 'strict',
                      'approvedWorker': args.allowed_worker,
                      'workerCredentialInjection': False, 'checks': results,
                      'scope': 'Paired TCP reachability only; not startup, TLS, UDP DNS, expiry, kernel/filesystem or E14 identity proof'}, indent=2))
finally:
    if created:
        kube('delete', 'deployment', name, '--wait=true', '--timeout=60s')
