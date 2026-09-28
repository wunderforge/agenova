# Copyright 2026 Dapeng Zhang and Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
locals {
  name = var.cluster_name
  azs  = slice(data.aws_availability_zones.available.names, 0, 2)
}
resource "aws_vpc" "demo" {
  cidr_block           = "10.82.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true
  tags                 = { Name = local.name }
  depends_on           = [terraform_data.guard]
}
resource "aws_internet_gateway" "demo" { vpc_id = aws_vpc.demo.id }
resource "aws_subnet" "demo" {
  count                   = 2
  vpc_id                  = aws_vpc.demo.id
  cidr_block              = cidrsubnet(aws_vpc.demo.cidr_block, 8, count.index)
  availability_zone       = local.azs[count.index]
  map_public_ip_on_launch = true
  tags                    = { Name = "${local.name}-${count.index}" }
}
resource "aws_route_table" "demo" {
  vpc_id = aws_vpc.demo.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.demo.id
  }
}
resource "aws_route_table_association" "demo" {
  count          = 2
  subnet_id      = aws_subnet.demo[count.index].id
  route_table_id = aws_route_table.demo.id
}
resource "aws_iam_role" "cluster" {
  name = "${local.name}-cluster"
  assume_role_policy = jsonencode({ Version = "2012-10-17", Statement = [{
    Effect = "Allow", Principal = { Service = "eks.amazonaws.com" }, Action = "sts:AssumeRole"
  }] })
  depends_on = [terraform_data.guard]
}
resource "aws_iam_role_policy_attachment" "cluster" {
  role       = aws_iam_role.cluster.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
}
resource "aws_cloudwatch_log_group" "eks" {
  name              = "/aws/eks/${local.name}/cluster"
  retention_in_days = 7
  depends_on        = [terraform_data.guard]
}
resource "aws_eks_cluster" "demo" {
  name                      = local.name
  role_arn                  = aws_iam_role.cluster.arn
  version                   = var.kubernetes_version
  enabled_cluster_log_types = ["api", "audit", "authenticator"]
  access_config {
    authentication_mode                         = "API"
    bootstrap_cluster_creator_admin_permissions = false
  }
  upgrade_policy { support_type = "STANDARD" }
  vpc_config {
    subnet_ids              = aws_subnet.demo[*].id
    endpoint_private_access = true
    endpoint_public_access  = true
    public_access_cidrs     = var.api_cidrs
  }
  depends_on = [aws_iam_role_policy_attachment.cluster, aws_cloudwatch_log_group.eks, aws_route_table_association.demo]
}
resource "aws_eks_access_entry" "operator" {
  cluster_name  = aws_eks_cluster.demo.name
  principal_arn = var.operator_arn
  type          = "STANDARD"
  lifecycle {
    precondition {
      condition     = split(":", var.operator_arn)[4] == var.account_id
      error_message = "Operator must belong to the selected team account."
    }
  }
}
resource "aws_eks_access_policy_association" "operator" {
  cluster_name  = aws_eks_cluster.demo.name
  principal_arn = aws_eks_access_entry.operator.principal_arn
  policy_arn    = "arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy"
  access_scope { type = "cluster" }
}
resource "aws_iam_role" "node" {
  name = "${local.name}-node"
  assume_role_policy = jsonencode({ Version = "2012-10-17", Statement = [{
    Effect = "Allow", Principal = { Service = "ec2.amazonaws.com" }, Action = "sts:AssumeRole"
  }] })
  depends_on = [terraform_data.guard]
}
resource "aws_iam_role_policy_attachment" "node" {
  for_each   = toset(["AmazonEKSWorkerNodePolicy", "AmazonEC2ContainerRegistryPullOnly", "AmazonEKS_CNI_Policy"])
  role       = aws_iam_role.node.name
  policy_arn = "arn:aws:iam::aws:policy/${each.value}"
}
resource "aws_launch_template" "node" {
  name_prefix   = "${local.name}-"
  instance_type = "t3.medium"
  metadata_options {
    http_endpoint               = "enabled"
    http_tokens                 = "required"
    http_put_response_hop_limit = 1
  }
  credit_specification { cpu_credits = "standard" }
  block_device_mappings {
    device_name = "/dev/xvda"
    ebs {
      volume_size           = 20
      volume_type           = "gp3"
      encrypted             = true
      delete_on_termination = true
    }
  }
  tag_specifications {
    resource_type = "instance"
    tags          = { Name = local.name, Project = "agenova", ExpiresAt = var.expires_at }
  }
  depends_on = [terraform_data.guard]
}
resource "aws_eks_node_group" "demo" {
  cluster_name    = aws_eks_cluster.demo.name
  node_group_name = "demo"
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.demo[*].id
  ami_type        = "AL2023_x86_64_STANDARD"
  capacity_type   = "ON_DEMAND"
  launch_template {
    id      = aws_launch_template.node.id
    version = aws_launch_template.node.latest_version
  }
  scaling_config {
    desired_size = 1
    min_size     = 1
    max_size     = 1
  }
  update_config { max_unavailable = 1 }
  depends_on = [aws_iam_role_policy_attachment.node]
}
resource "aws_ecr_repository" "demo" {
  for_each             = toset(["control-plane", "worker"])
  name                 = "${local.name}/${each.value}"
  image_tag_mutability = "IMMUTABLE"
  force_delete         = false
  image_scanning_configuration { scan_on_push = true }
  encryption_configuration { encryption_type = "AES256" }
  depends_on = [terraform_data.guard]
}
resource "aws_budgets_budget" "demo" {
  name         = "${local.name}-account-monthly"
  budget_type  = "COST"
  limit_amount = tostring(var.budget_usd)
  limit_unit   = "USD"
  time_unit    = "MONTHLY"
  dynamic "notification" {
    for_each = [50, 80, 100]
    content {
      comparison_operator        = "GREATER_THAN"
      threshold                  = notification.value
      threshold_type             = "PERCENTAGE"
      notification_type          = "ACTUAL"
      subscriber_email_addresses = [var.budget_email]
    }
  }
  notification {
    comparison_operator        = "GREATER_THAN"
    threshold                  = 100
    threshold_type             = "PERCENTAGE"
    notification_type          = "FORECASTED"
    subscriber_email_addresses = [var.budget_email]
  }
  depends_on = [terraform_data.guard]
}
