variable "environment" {
  type        = string
  description = "The environment deploy to (e.g. prod, staging)"
}

variable "alb_arn" {
  type        = string
  description = "ARN of the ALB to associate WAF with"
}
