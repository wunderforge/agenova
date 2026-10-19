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
variable "account_id" {
  type = string
}
variable "public_origin" {
  type    = string
  default = "https://app.agenova.app"
  validation {
    condition     = can(regex("^https://[a-z0-9.-]+$", var.public_origin))
    error_message = "Provide the HTTPS service origin without a path or credentials."
  }
}
variable "login_domain_prefix" {
  type    = string
  default = "agenova-team-931228356546"
}
provider "aws" {
  region              = "ap-southeast-2"
  allowed_account_ids = [var.account_id]
  default_tags {
    tags = { Project = "agenova", Purpose = "production-identity", ManagedBy = "terraform" }
  }
}
data "aws_caller_identity" "current" {}
resource "terraform_data" "non_root" {
  lifecycle {
    precondition {
      condition     = !endswith(data.aws_caller_identity.current.arn, ":root")
      error_message = "Use a designated non-root deployment operator."
    }
  }
}
resource "aws_cognito_user_pool" "team" {
  name                     = "agenova-production"
  deletion_protection      = "ACTIVE"
  user_pool_tier           = "ESSENTIALS"
  username_attributes      = ["email"]
  auto_verified_attributes = ["email"]
  mfa_configuration        = "ON"
  software_token_mfa_configuration { enabled = true }
  admin_create_user_config {
    allow_admin_create_user_only = true
    invite_message_template {
      email_subject = "Your Agenova invitation"
      email_message = "You were invited to Agenova. Sign in at ${var.public_origin}/login with username {username} and temporary password {####}. Set your own password and authenticator when prompted."
    }
  }
  password_policy {
    minimum_length                   = 14
    require_lowercase                = true
    require_uppercase                = true
    require_numbers                  = true
    require_symbols                  = true
    temporary_password_validity_days = 7
  }
  account_recovery_setting {
    recovery_mechanism {
      name     = "verified_email"
      priority = 1
    }
  }
  user_attribute_update_settings { attributes_require_verification_before_update = ["email"] }
  email_configuration { email_sending_account = "COGNITO_DEFAULT" }
  lifecycle { prevent_destroy = true }
  depends_on = [terraform_data.non_root]
}
resource "aws_cognito_resource_server" "api" {
  identifier   = "agenova"
  name         = "Agenova application API"
  user_pool_id = aws_cognito_user_pool.team.id
  scope {
    scope_name        = "api"
    scope_description = "Authenticate to Agenova; project membership separately authorizes operations"
  }
}
resource "aws_cognito_user_pool_client" "member" {
  for_each                             = toset(["native", "web"])
  name                                 = "agenova-${each.key}"
  user_pool_id                         = aws_cognito_user_pool.team.id
  generate_secret                      = false
  allowed_oauth_flows_user_pool_client = true
  allowed_oauth_flows                  = ["code"]
  allowed_oauth_scopes                 = ["openid", "email", "${aws_cognito_resource_server.api.identifier}/api"]
  supported_identity_providers         = ["COGNITO"]
  callback_urls                        = each.key == "native" ? ["http://127.0.0.1:38123/callback"] : ["${var.public_origin}/auth/callback"]
  logout_urls                          = ["${var.public_origin}/login"]
  explicit_auth_flows                  = ["ALLOW_REFRESH_TOKEN_AUTH"]
  prevent_user_existence_errors        = "ENABLED"
  enable_token_revocation              = true
  access_token_validity                = 5
  id_token_validity                    = 5
  refresh_token_validity               = 1
  read_attributes                      = ["email", "email_verified"]
  write_attributes                     = ["email"]
  token_validity_units {
    access_token  = "minutes"
    id_token      = "minutes"
    refresh_token = "days"
  }
}
resource "aws_cognito_user_pool_domain" "login" {
  domain                = var.login_domain_prefix
  user_pool_id          = aws_cognito_user_pool.team.id
  managed_login_version = 2
}
resource "aws_cognito_managed_login_branding" "member" {
  for_each                    = aws_cognito_user_pool_client.member
  client_id                   = each.value.id
  user_pool_id                = aws_cognito_user_pool.team.id
  use_cognito_provided_values = true
}
output "user_pool_id" { value = aws_cognito_user_pool.team.id }
output "issuer" { value = "https://cognito-idp.ap-southeast-2.amazonaws.com/${aws_cognito_user_pool.team.id}" }
output "native_client_id" { value = aws_cognito_user_pool_client.member["native"].id }
output "web_client_id" { value = aws_cognito_user_pool_client.member["web"].id }
output "oauth_origin" { value = "https://${aws_cognito_user_pool_domain.login.domain}.auth.ap-southeast-2.amazoncognito.com" }
