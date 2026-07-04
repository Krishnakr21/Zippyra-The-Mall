output "proxy_endpoint" {
  value       = aws_db_proxy.main.endpoint
  description = "RDS Proxy endpoint — services connect HERE, not to RDS directly"
}

output "proxy_arn" {
  value       = aws_db_proxy.main.arn
  description = "RDS Proxy ARN"
}
