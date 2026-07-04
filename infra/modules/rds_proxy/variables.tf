variable "environment" {
  type        = string
  description = "The environment to deploy to (e.g. prod, staging)"
}

variable "vpc_id" {
  type        = string
  description = "VPC ID where the RDS Proxy will be deployed"
}

variable "private_subnet_ids" {
  type        = list(string)
  description = "List of private subnet IDs for the RDS Proxy"
}

variable "eks_nodes_sg_ids" {
  type        = list(string)
  description = "Security group IDs of EKS worker nodes allowed to connect"
}

variable "db_instance_identifier" {
  type        = string
  description = "RDS instance identifier to proxy connections to"
}

variable "db_secret_arn" {
  type        = string
  description = "ARN of the Secrets Manager secret containing DB credentials"
}

variable "kms_key_arn" {
  type        = string
  description = "ARN of the KMS key used to encrypt the DB secret"
}
