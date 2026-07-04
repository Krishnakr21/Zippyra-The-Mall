variable "environment" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "instance_class" {
  type    = string
  default = "db.t4g.large" # pilot sizing per spec
}

variable "allocated_storage" {
  type    = number
  default = 100
}

variable "max_allocated_storage" {
  type    = number
  default = 500
}

variable "db_password" {
  type      = string
  sensitive = true
}

variable "replica_count" {
  type    = number
  default = 2 # spec: 2 read replicas at pilot
}
