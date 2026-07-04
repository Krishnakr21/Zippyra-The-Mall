output "db_endpoint" {
  value       = aws_db_instance.main.endpoint
  description = "Primary RDS endpoint"
}

output "db_reader_endpoints" {
  value       = aws_db_instance.replica[*].endpoint
  description = "Read replica endpoints"
}

output "db_name" {
  value = aws_db_instance.main.db_name
}

output "db_port" {
  value = aws_db_instance.main.port
}
