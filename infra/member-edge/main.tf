# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
terraform {
  required_version = ">= 1.9.0, < 2.0.0"
  required_providers { aws = { source = "hashicorp/aws", version = "~> 6.38" } }
}
provider "aws" {
  region              = "ap-southeast-2"
  allowed_account_ids = ["931228356546"]
}
data "aws_lb" "existing" { name = "agenova-demo" }
data "aws_lb_listener" "existing" {
  load_balancer_arn = data.aws_lb.existing.arn
  port              = 443
}
data "aws_eks_cluster" "existing" { name = "agenova-demo" }
data "aws_eks_node_group" "existing" {
  cluster_name    = data.aws_eks_cluster.existing.name
  node_group_name = "demo"
}
resource "aws_lb_target_group" "members" {
  name                 = "agenova-members"
  vpc_id               = data.aws_eks_cluster.existing.vpc_config[0].vpc_id
  port                 = 31090
  protocol             = "HTTP"
  target_type          = "instance"
  deregistration_delay = 10
  health_check {
    path              = "/healthz"
    matcher           = "200"
    interval          = 15
    healthy_threshold = 2
  }
}
resource "aws_autoscaling_attachment" "members" {
  autoscaling_group_name = data.aws_eks_node_group.existing.resources[0].autoscaling_groups[0].name
  lb_target_group_arn    = aws_lb_target_group.members.arn
}
resource "aws_lb_listener_rule" "members" {
  listener_arn = data.aws_lb_listener.existing.arn
  priority     = 215
  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.members.arn
  }
  condition {
    host_header { values = ["app.agenova.app"] }
  }
}
resource "aws_vpc_security_group_ingress_rule" "members" {
  security_group_id            = data.aws_eks_cluster.existing.vpc_config[0].cluster_security_group_id
  referenced_security_group_id = one(data.aws_lb.existing.security_groups)
  ip_protocol                  = "tcp"
  from_port                    = 31090
  to_port                      = 31090
}
resource "aws_vpc_security_group_egress_rule" "members" {
  security_group_id            = one(data.aws_lb.existing.security_groups)
  referenced_security_group_id = data.aws_eks_cluster.existing.vpc_config[0].cluster_security_group_id
  ip_protocol                  = "tcp"
  from_port                    = 31090
  to_port                      = 31090
}
resource "aws_route53_record" "members" {
  zone_id = "Z00685831HNHNRLEZ9VYK"
  name    = "app.agenova.app"
  type    = "A"
  alias {
    name                   = data.aws_lb.existing.dns_name
    zone_id                = data.aws_lb.existing.zone_id
    evaluate_target_health = false
  }
}
