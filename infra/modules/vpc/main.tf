variable "environment" {
  type        = string
  description = "Environment name (e.g., pilot, production)"
}

variable "vpc_cidr" {
  type        = string
  description = "VPC CIDR block"
}

output "vpc_id" {
  value = "vpc-dummy-${var.environment}"
}
