# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
import unittest
from review_plan import review


def plan(address, actions, mode='managed'):
    return {'format_version': '1.2', 'resource_changes': [
        {'address': address, 'mode': mode, 'change': {'actions': actions}}]}


class PlanReviewTests(unittest.TestCase):
    def test_ephemeral_create_and_delete(self):
        self.assertTrue(review(plan('aws_eks_cluster.demo', ['create']), 'production/eks', 'on'))
        self.assertTrue(review(plan('aws_eks_node_group.demo', ['delete']), 'production/eks', 'off'))

    def test_cluster_scoped_pod_identity_lifecycle(self):
        for address in ['aws_eks_addon.pod_identity', 'aws_eks_pod_identity_association.model_gateway']:
            self.assertTrue(review(plan(address, ['create']), 'production/eks', 'on'))
            self.assertTrue(review(plan(address, ['delete']), 'production/eks', 'off'))

    def test_protect_persistent_and_unknown_changes(self):
        for address in ['aws_vpc.demo', 'aws_subnet.demo[0]', 'aws_iam_role.node',
                        'aws_ecr_repository.demo["worker"]', 'aws_db_instance.application',
                        'aws_eks_cluster.other']:
            for action in [['delete'], ['create'], ['update'], ['delete', 'create']]:
                with self.subTest(address=address, action=action), self.assertRaises(ValueError):
                    review(plan(address, action), 'production/eks', 'on')

    def test_wrong_direction_replacement_and_persistent_root(self):
        for root, op, actions in [('database', 'off', ['delete']),
                                  ('production/eks', 'on', ['delete', 'create']),
                                  ('production/eks', 'off', ['create'])]:
            with self.assertRaises(ValueError):
                review(plan('aws_eks_cluster.demo', actions), root, op)

    def test_incomplete_and_missing_format_fail(self):
        for value in [{}, {'format_version': '2.0'}, {'format_version': '1.2', 'complete': False},
                      {'format_version': '1.2', 'errored': True}]:
            with self.assertRaises(ValueError):
                review(value, 'production/eks', 'on')

    def test_data_reads_and_unchanged_persistent_resources(self):
        self.assertEqual(review(plan('aws_vpc.demo', ['no-op']), 'production/eks', 'on'), [])
        self.assertEqual(review(plan('data.aws_eks_cluster.demo', ['read'], 'data'), 'production/edge', 'on'), [])


if __name__ == '__main__':
    unittest.main()
