variable "environment" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "node_type" {
  type    = string
  default = "cache.t4g.medium" # pilot sizing per spec
}

variable "num_cache_nodes" {
  type    = number
  default = 2 # pilot: primary + 1 replica
}

variable "redis_auth_token" {
  type      = string
  sensitive = true
}
