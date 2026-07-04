variable "environment" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "eks_role_arn" {
  type        = string
  description = "EKS cluster IAM role ARN from iam module"
}

variable "cluster_version" {
  type    = string
  default = "1.29"
}

variable "node_instance_type" {
  type    = string
  default = "t3.medium" # pilot sizing per spec
}

variable "node_desired_size" {
  type    = number
  default = 3
}

variable "node_min_size" {
  type    = number
  default = 2
}

variable "node_max_size" {
  type    = number
  default = 6
}

variable "public_access_cidrs" {
  type        = list(string)
  description = "CIDRs allowed to access EKS public endpoint - restrict to your office/VPN IPs"
  default     = ["0.0.0.0/0"] # tighten this before go-live
}
