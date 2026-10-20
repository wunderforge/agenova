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

# Owner-managed zone and domain registration are intentionally not resources.
data "aws_route53_zone" "existing" {
  zone_id = "Z00685831HNHNRLEZ9VYK"
}

data "aws_caller_identity" "current" {}

resource "aws_acm_certificate" "demo" {
  domain_name               = "agenova.app"
  subject_alternative_names = ["*.agenova.app"]
  validation_method         = "DNS"
  tags = {
    Project   = "agenova-demo"
    Purpose   = "demo-https"
    ManagedBy = "terraform"
  }
  lifecycle {
    prevent_destroy = true
    precondition {
      condition     = !endswith(data.aws_caller_identity.current.arn, ":root") && trimsuffix(data.aws_route53_zone.existing.name, ".") == "agenova.app" && !data.aws_route53_zone.existing.private_zone
      error_message = "Use a non-root operator and the existing public agenova.app zone."
    }
  }
}

resource "aws_route53_record" "validation" {
  for_each = {
    for option in aws_acm_certificate.demo.domain_validation_options : option.domain_name => {
      name  = option.resource_record_name
      type  = option.resource_record_type
      value = option.resource_record_value
    } if option.domain_name == "agenova.app"
  }
  zone_id = data.aws_route53_zone.existing.zone_id
  name    = each.value.name
  type    = each.value.type
  ttl     = 300
  records = [each.value.value]
  lifecycle {
    prevent_destroy = true
  }
}

# Enable only after public delegation works, avoiding an unbounded issuance wait.
variable "wait_for_certificate" {
  type    = bool
  default = false
}
resource "aws_acm_certificate_validation" "demo" {
  count                   = var.wait_for_certificate ? 1 : 0
  certificate_arn         = aws_acm_certificate.demo.arn
  validation_record_fqdns = [for record in aws_route53_record.validation : record.fqdn]
  timeouts {
    create = "5m"
  }
}
output "certificate_arn" {
  value = aws_acm_certificate.demo.arn
}
output "hosted_zone_id" {
  value = data.aws_route53_zone.existing.zone_id
}

# Retained historical verification record; not used by the new certificate.
resource "aws_route53_record" "legacy_validation" {
  zone_id = data.aws_route53_zone.existing.zone_id
  name    = "_2072119b633521d8f4e042dc1dbab723.demo.agenova.app."
  type    = "CNAME"
  ttl     = 300
  records = ["_18d2ca087adc2a953661695f7accc2f5.wzccmgtwzk.acm-validations.aws."]
  lifecycle { prevent_destroy = true }
}

provider "aws" {
  alias               = "sydney"
  region              = "ap-southeast-2"
  allowed_account_ids = ["931228356546"]
}
resource "aws_acm_certificate" "sydney" {
  provider                  = aws.sydney
  domain_name               = "agenova.app"
  subject_alternative_names = ["*.agenova.app"]
  validation_method         = "DNS"
  tags                      = { Project = "agenova-demo", ManagedBy = "terraform" }
  lifecycle { prevent_destroy = true }
}
resource "aws_acm_certificate_validation" "sydney" {
  provider                = aws.sydney
  certificate_arn         = aws_acm_certificate.sydney.arn
  validation_record_fqdns = [for record in aws_route53_record.validation : record.fqdn]
  timeouts { create = "5m" }
}
output "sydney_certificate_arn" { value = aws_acm_certificate.sydney.arn }
