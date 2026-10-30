#!/usr/bin/env python3
# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
"""Review Terraform plan JSON for routine EKS on/off operations; never applies it."""
import argparse
import json
import sys

ALLOWED = {
    'production/eks': {
        'aws_eks_cluster.demo', 'aws_eks_node_group.demo',
        'aws_eks_access_entry.operator', 'aws_eks_access_policy_association.operator',
        'aws_eks_addon.vpc_cni',
    },
    'production/edge': {
        'aws_lb.demo', 'aws_lb_listener.https', 'aws_lb_target_group.demo',
        'aws_autoscaling_attachment.demo', 'aws_route53_record.demo',
    },
    'member-edge': {
        'aws_lb_target_group.members', 'aws_lb_listener_rule.members',
        'aws_autoscaling_attachment.members', 'aws_route53_record.members',
    },
}

def review(plan, root, operation):
    if root not in ALLOWED:
        raise ValueError('Unknown or persistent root: routine lifecycle is prohibited')
    if operation not in ('on', 'off'):
        raise ValueError('Expected on or off')
    if not str(plan.get('format_version', '')).startswith('1.'):
        raise ValueError('Unsupported/missing plan format')
    if plan.get('errored') or plan.get('complete') is False or plan.get('deferred_changes'):
        raise ValueError('Errored, incomplete or deferred plan')
    changes = []
    for item in plan.get('resource_changes', []):
        actions = item['change']['actions']
        if actions == ['no-op'] or (item.get('mode') == 'data' and actions == ['read']):
            continue
        address = item['address']
        if item.get('mode') != 'managed' or address not in ALLOWED[root]:
            raise ValueError(f'Non-ephemeral or unknown resource change: {address}')
        permitted = (['create'], ['update']) if operation == 'on' else (['delete'],)
        if actions not in permitted:
            raise ValueError(f'Unexpected {operation} action on {address}: {actions}')
        changes.append((address, actions))
    return changes

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', required=True, choices=sorted(ALLOWED))
    parser.add_argument('--operation', required=True, choices=['on', 'off'])
    parser.add_argument('plan_json')
    args = parser.parse_args()
    try:
        with open(args.plan_json, encoding='utf-8') as f:
            changes = review(json.load(f), args.root, args.operation)
        for address, actions in changes:
            print(address, ','.join(actions))
        print('Scope check passed. Verify account, remote backend, inputs and saved plan before applying.')
        return 0
    except (ValueError, KeyError, TypeError, OSError) as error:
        print(f'REJECTED: {error}', file=sys.stderr)
        return 1

if __name__ == '__main__':
    sys.exit(main())
