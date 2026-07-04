variable "environment" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "number_of_broker_nodes" {
  type    = number
  default = 3 # pilot: 3 brokers per spec
}

variable "broker_instance_type" {
  type    = string
  default = "kafka.t3.small" # pilot sizing per spec
}

variable "broker_volume_size" {
  type    = number
  default = 100
}
