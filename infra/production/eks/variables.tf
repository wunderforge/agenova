# Copyright 2026 Dapeng Zhang and Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
variable "account_id" {
  type = string
  validation {
    condition     = can(regex("^[0-9]{12}$", var.account_id))
    error_message = "Specify the intended 12-digit team AWS account ID."
  }
}
variable "operator_arn" {
  type        = string
  description = "Existing IAM role or user ARN; not root or an STS session ARN."
  validation {
    condition     = can(regex("^arn:aws:iam::[0-9]{12}:(role|user)/.+$", var.operator_arn))
    error_message = "An existing IAM user/role ARN is required."
  }
}
variable "api_cidrs" {
  type        = list(string)
  description = "Operator IPv4 addresses allowed to reach the Kubernetes API."
  validation {
    condition     = length(var.api_cidrs) > 0 && alltrue([for c in var.api_cidrs : can(cidrnetmask(c)) && try(tonumber(split("/", c)[1]) >= 24, false)])
    error_message = "Provide explicit IPv4 CIDRs with /24 or narrower prefixes (prefer /32)."
  }
}
variable "cluster_name" {
  type    = string
  default = "agenova-demo"
  validation {
    condition     = can(regex("^agenova-[a-z0-9-]{1,24}$", var.cluster_name))
    error_message = "Use an agenova- prefix and at most 24 suffix characters."
  }
}
variable "kubernetes_version" {
  type        = string
  default     = "1.35"
  description = "Verify standard support and Agent Sandbox compatibility before apply."
}
variable "expires_at" {
  type        = string
  description = "UTC cleanup deadline. Tag is documentation, NOT automatic deletion."
  validation {
    condition     = can(formatdate("YYYY-MM-DD", var.expires_at))
    error_message = "Provide an RFC3339 UTC cleanup deadline."
  }
}
variable "budget_usd" {
  type        = number
  default     = 50
  description = "Conservative USD alert threshold for AUD 100/month; verify exchange rate and tax. Not a spending cap."
  validation {
    condition     = var.budget_usd > 0 && var.budget_usd <= 50
    error_message = "This demo limits the monthly alert threshold to USD 50."
  }
}
variable "budget_email" {
  type = string
  validation {
    condition     = can(regex("^[^@ ]+@[^@ ]+\\.[^@ ]+$", var.budget_email))
    error_message = "Provide the budget notification recipient."
  }
}
