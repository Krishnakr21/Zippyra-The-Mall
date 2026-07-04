variable "environment" {
  type = string
}

variable "kms_key_arn" {
  type = string
}

variable "db_password" {
  type      = string
  sensitive = true
}

variable "redis_auth_token" {
  type      = string
  sensitive = true
}
