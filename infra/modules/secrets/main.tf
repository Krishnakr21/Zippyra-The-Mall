# ── RDS Password Secret ──────────────────────────────────────────────────────
resource "aws_secretsmanager_secret" "db_password" {
  name        = "${var.environment}/zippyra/db-password"
  description = "RDS main password for ${var.environment}"
  kms_key_id  = var.kms_key_arn
  tags        = { Name = "${var.environment}-db-password" }
}

resource "aws_secretsmanager_secret_version" "db_password" {
  secret_id     = aws_secretsmanager_secret.db_password.id
  secret_string = jsonencode({
    username = "zippyra_admin"
    password = var.db_password
    engine   = "postgres"
    host     = "${var.environment}-zippyra-postgres.cluster-xyz.ap-south-1.rds.amazonaws.com"
    port     = 5432
  })
}

# ── Redis Auth Token Secret ──────────────────────────────────────────────────
resource "aws_secretsmanager_secret" "redis_auth" {
  name        = "${var.environment}/zippyra/redis-auth"
  description = "Redis Auth Token for ${var.environment}"
  kms_key_id  = var.kms_key_arn
  tags        = { Name = "${var.environment}-redis-auth" }
}

resource "aws_secretsmanager_secret_version" "redis_auth" {
  secret_id     = aws_secretsmanager_secret.redis_auth.id
  secret_string = var.redis_auth_token
}

# ── JWT Ed25519 Key Pair ─────────────────────────────────────────────────────
resource "aws_secretsmanager_secret" "jwt_private" {
  name        = "${var.environment}/zippyra/jwt-private"
  description = "Ed25519 JWT Private Key"
  kms_key_id  = var.kms_key_arn
  tags        = { Name = "${var.environment}-jwt-private" }
}

resource "aws_secretsmanager_secret" "jwt_public" {
  name        = "${var.environment}/zippyra/jwt-public"
  description = "Ed25519 JWT Public Key"
  kms_key_id  = var.kms_key_arn
  tags        = { Name = "${var.environment}-jwt-public" }
}

# Note: Actual key values should be pushed via CLI post-infra-apply as per plan
# or managed via a bootstrap script. We define the containers here.
