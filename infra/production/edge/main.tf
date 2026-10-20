# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
terraform {
  required_version = ">= 1.9.0, < 2.0.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.38"
    }
  }
}
provider "aws" {
  region              = "us-east-1"
  allowed_account_ids = ["931228356546"]
}
data "aws_route53_zone" "existing" { zone_id = "Z00685831HNHNRLEZ9VYK" }
resource "aws_route53_record" "demo" {
  zone_id = data.aws_route53_zone.existing.zone_id
  name    = "demo.agenova.app"
  type    = "A"
  alias {
    name                   = aws_lb.demo.dns_name
    zone_id                = aws_lb.demo.zone_id
    evaluate_target_health = false
  }
  depends_on = [aws_lb_listener.https]
}
moved {
  from = aws_route53_record.demo["A"]
  to   = aws_route53_record.demo
}
output "url" { value = "https://demo.agenova.app/?mode=connected" }
