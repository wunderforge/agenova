# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
provider "aws" {
  alias               = "sydney"
  region              = "ap-southeast-2"
  allowed_account_ids = ["931228356546"]
}
data "aws_eks_cluster" "demo" {
  provider = aws.sydney
  name     = "agenova-demo"
}
data "aws_eks_node_group" "demo" {
  provider        = aws.sydney
  cluster_name    = data.aws_eks_cluster.demo.name
  node_group_name = "demo"
}
data "aws_acm_certificate" "sydney" {
  provider    = aws.sydney
  domain      = "agenova.app"
  statuses    = ["ISSUED"]
  most_recent = true
}
resource "aws_security_group" "alb" {
  provider    = aws.sydney
  name_prefix = "agenova-demo-alb-"
  vpc_id      = data.aws_eks_cluster.demo.vpc_config[0].vpc_id
  description = "HTTPS public entry to authenticated Agenova demo"
  tags        = { Project = "agenova-demo", ManagedBy = "terraform" }
}
resource "aws_vpc_security_group_ingress_rule" "https" {
  provider          = aws.sydney
  security_group_id = aws_security_group.alb.id
  cidr_ipv4         = "0.0.0.0/0"
  from_port         = 443
  to_port           = 443
  ip_protocol       = "tcp"
}
resource "aws_vpc_security_group_egress_rule" "backend" {
  provider                     = aws.sydney
  security_group_id            = aws_security_group.alb.id
  referenced_security_group_id = data.aws_eks_cluster.demo.vpc_config[0].cluster_security_group_id
  from_port                    = 31089
  to_port                      = 31089
  ip_protocol                  = "tcp"
}
resource "aws_vpc_security_group_ingress_rule" "backend" {
  provider                     = aws.sydney
  security_group_id            = data.aws_eks_cluster.demo.vpc_config[0].cluster_security_group_id
  referenced_security_group_id = aws_security_group.alb.id
  from_port                    = 31089
  to_port                      = 31089
  ip_protocol                  = "tcp"
}
resource "aws_lb" "demo" {
  provider                   = aws.sydney
  name                       = "agenova-demo"
  load_balancer_type         = "application"
  internal                   = false
  ip_address_type            = "ipv4"
  security_groups            = [aws_security_group.alb.id]
  subnets                    = data.aws_eks_node_group.demo.subnet_ids
  idle_timeout               = 300
  drop_invalid_header_fields = true
  tags                       = { Project = "agenova-demo", ManagedBy = "terraform", ExpiresAt = "2026-09-28T15:00:00Z" }
}
resource "aws_lb_target_group" "demo" {
  provider             = aws.sydney
  name                 = "agenova-demo-portal"
  vpc_id               = data.aws_eks_cluster.demo.vpc_config[0].vpc_id
  port                 = 31089
  protocol             = "HTTP"
  target_type          = "instance"
  deregistration_delay = 10
  health_check {
    path                = "/healthz"
    matcher             = "200"
    interval            = 15
    healthy_threshold   = 2
    unhealthy_threshold = 2
  }
}
# ASG attachment keeps targets current when the managed node is replaced.
resource "aws_autoscaling_attachment" "demo" {
  provider               = aws.sydney
  autoscaling_group_name = data.aws_eks_node_group.demo.resources[0].autoscaling_groups[0].name
  lb_target_group_arn    = aws_lb_target_group.demo.arn
}
resource "aws_lb_listener" "https" {
  provider          = aws.sydney
  load_balancer_arn = aws_lb.demo.arn
  port              = 443
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS13-1-2-2021-06"
  certificate_arn   = data.aws_acm_certificate.sydney.arn
  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.demo.arn
  }
}
output "alb_dns_name" { value = aws_lb.demo.dns_name }
output "target_group_arn" { value = aws_lb_target_group.demo.arn }
