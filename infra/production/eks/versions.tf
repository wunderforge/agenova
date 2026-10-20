# Copyright 2026 Dapeng Zhang and Agenova contributors.
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
  region              = "ap-southeast-2"
  allowed_account_ids = [var.account_id]
  default_tags {
    tags = {
      Project   = "agenova"
      Purpose   = "disposable-team-demo"
      ManagedBy = "terraform"
      ExpiresAt = var.expires_at
    }
  }
}
data "aws_caller_identity" "current" {}
data "aws_availability_zones" "available" { state = "available" }
resource "terraform_data" "guard" {
  lifecycle {
    precondition {
      condition     = !endswith(data.aws_caller_identity.current.arn, ":root")
      error_message = "Use an authenticated non-root operator session."
    }
  }
}
