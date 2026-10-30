# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
terraform {
  required_version = ">= 1.10.0, < 2.0.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.38"
    }
  }
}
variable "account_id" { type = string }
variable "vpc_id" { type = string }
variable "subnet_ids" {
  type = set(string)
  validation {
    condition     = length(var.subnet_ids) >= 2
    error_message = "RDS requires subnets in at least two availability zones, even for Single-AZ."
  }
}
variable "application_security_group_id" { type = string }
provider "aws" {
  region              = "ap-southeast-2"
  allowed_account_ids = [var.account_id]
  default_tags {
    tags = { Project = "agenova", Purpose = "production-data", ManagedBy = "terraform" }
  }
}
data "aws_caller_identity" "current" {}
data "aws_rds_engine_version" "postgres" {
  engine                 = "postgres"
  parameter_group_family = "postgres17"
  latest                 = true
}
resource "aws_db_subnet_group" "private" {
  name       = "agenova-production"
  subnet_ids = var.subnet_ids
}
resource "aws_security_group" "database" {
  name        = "agenova-production-postgres"
  description = "PostgreSQL from the designated application host only"
  vpc_id      = var.vpc_id
}
resource "aws_vpc_security_group_ingress_rule" "application" {
  security_group_id            = aws_security_group.database.id
  referenced_security_group_id = var.application_security_group_id
  ip_protocol                  = "tcp"
  from_port                    = 5432
  to_port                      = 5432
}
resource "aws_db_parameter_group" "postgres" {
  name   = "agenova-production-postgres17"
  family = "postgres17"
  parameter {
    name         = "rds.force_ssl"
    value        = "1"
    apply_method = "immediate"
  }
}
resource "aws_db_instance" "application" {
  identifier                   = "agenova-production"
  engine                       = "postgres"
  engine_version               = data.aws_rds_engine_version.postgres.version
  instance_class               = "db.t4g.micro"
  allocated_storage            = 20
  max_allocated_storage        = 0
  storage_type                 = "gp3"
  storage_encrypted            = true
  multi_az                     = false
  publicly_accessible          = false
  db_subnet_group_name         = aws_db_subnet_group.private.name
  vpc_security_group_ids       = [aws_security_group.database.id]
  parameter_group_name         = aws_db_parameter_group.postgres.name
  db_name                      = "agenova"
  username                     = "agenova_operator"
  manage_master_user_password  = true
  backup_retention_period      = 0
  delete_automated_backups     = true
  skip_final_snapshot          = true
  deletion_protection          = true
  auto_minor_version_upgrade   = true
  allow_major_version_upgrade  = false
  performance_insights_enabled = false
  monitoring_interval          = 0
  lifecycle {
    prevent_destroy = true
    precondition {
      condition     = !endswith(data.aws_caller_identity.current.arn, ":root")
      error_message = "Use a designated non-root deployment operator."
    }
  }
}
output "endpoint" { value = aws_db_instance.application.address }
output "master_secret_arn" { value = aws_db_instance.application.master_user_secret[0].secret_arn }
