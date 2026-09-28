# Copyright 2026 Dapeng Zhang and Agenova contributors.
# SPDX-License-Identifier: Apache-2.0

# Only the trusted installed control plane receives this identity. Workers use
# a different, unassociated service account and the node role has no InvokeModel.
resource "aws_eks_addon" "pod_identity" {
  cluster_name  = aws_eks_cluster.demo.name
  addon_name    = "eks-pod-identity-agent"
  addon_version = "v1.3.10-eksbuild.3"
  depends_on    = [aws_eks_node_group.demo]
}
resource "aws_iam_role" "model_gateway" {
  name = "${local.name}-model-gateway"
  assume_role_policy = jsonencode({ Version = "2012-10-17", Statement = [{
    Effect = "Allow", Principal = { Service = "pods.eks.amazonaws.com" },
    Action = ["sts:AssumeRole", "sts:TagSession"],
    Condition = { StringEquals = {
      "aws:SourceAccount"                         = var.account_id
      "aws:RequestTag/kubernetes-namespace"       = "agenova-system"
      "aws:RequestTag/kubernetes-service-account" = "agenova-control-plane"
    }, ArnEquals = { "aws:SourceArn" = aws_eks_cluster.demo.arn } }
  }] })
  depends_on = [terraform_data.guard]
}
resource "aws_iam_role_policy" "model_gateway" {
  name = "invoke-demo-model"
  role = aws_iam_role.model_gateway.id
  policy = jsonencode({ Version = "2012-10-17", Statement = [{
    Effect   = "Allow"
    Action   = "bedrock:InvokeModel"
    Resource = "arn:aws:bedrock:ap-southeast-2::foundation-model/amazon.nova-micro-v1:0"
  }] })
}
resource "aws_eks_pod_identity_association" "model_gateway" {
  cluster_name    = aws_eks_cluster.demo.name
  namespace       = "agenova-system"
  service_account = "agenova-control-plane"
  role_arn        = aws_iam_role.model_gateway.arn
  depends_on      = [aws_eks_addon.pod_identity, aws_iam_role_policy.model_gateway]
}
