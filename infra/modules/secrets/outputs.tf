output "db_secret_arn" {
  value = aws_secretsmanager_secret.db_password.arn
}

output "redis_secret_arn" {
  value = aws_secretsmanager_secret.redis_auth.arn
}

output "jwt_private_secret_arn" {
  value = aws_secretsmanager_secret.jwt_private.arn
}
