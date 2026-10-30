# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
# Private team state; authenticate with AWS_PROFILE (SSO), never inline keys.
terraform {
  backend "s3" {
    bucket              = "agenova-tfstate-931228356546-ap-southeast-2"
    key                 = "production/eks/terraform.tfstate"
    region              = "ap-southeast-2"
    encrypt             = true
    use_lockfile        = true
    allowed_account_ids = ["931228356546"]
  }
}
