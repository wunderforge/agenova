# Copyright 2026 Dapeng Zhang and Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
output "cluster_name" { value = aws_eks_cluster.demo.name }
output "region" { value = "ap-southeast-2" }
output "ecr_repositories" { value = { for k, r in aws_ecr_repository.demo : k => r.repository_url } }
output "cleanup_deadline" {
  value       = var.expires_at
  description = "Reminder only: this configuration does NOT schedule automatic destruction."
}
