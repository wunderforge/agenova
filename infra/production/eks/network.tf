# Copyright 2026 Dapeng Zhang and Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
# Apply deploy/production/eks/network-policies.yaml before adopting strict enforcement
# on an existing cluster. New clusters start fail-closed until those policies exist.
resource "aws_eks_addon" "vpc_cni" {
  cluster_name                = aws_eks_cluster.demo.name
  addon_name                  = "vpc-cni"
  addon_version               = "v1.22.4-eksbuild.3"
  resolve_conflicts_on_create = "OVERWRITE"
  resolve_conflicts_on_update = "PRESERVE"
  configuration_values = jsonencode({
    enableNetworkPolicy = "true"
    env                 = { NETWORK_POLICY_ENFORCING_MODE = "strict" }
  })
  depends_on = [aws_eks_node_group.demo]
}
