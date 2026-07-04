variable "aws_region" {
  type    = string
  default = "ap-south-1"
}

variable "environment" {
  type    = string
  default = "pilot"
}

variable "db_password" {
  type      = string
  sensitive = true
}

variable "redis_auth_token" {
  type      = string
  sensitive = true
}

variable "allowed_cidr_blocks" {
  type    = list(string)
  default = ["0.0.0.0/0"]
}

variable "alb_arn" {
  type    = string
  default = ""
}

# Enforce India region — data localization compliance
variable "aws_region_validation" {
  type    = string
  default = null

  validation {
    condition     = var.aws_region_validation == null || contains(["ap-south-1", "ap-south-2"], var.aws_region_validation)
    error_message = "Must use India AWS region for DPDP data localization compliance."
  }
}