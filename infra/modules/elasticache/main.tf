data "aws_subnets" "private" {
  filter {
    name   = "vpc-id"
    values = [var.vpc_id]
  }
  filter {
    name   = "tag:Name"
    values = ["${var.environment}-private-*"]
  }
}

data "aws_security_group" "elasticache" {
  filter {
    name   = "tag:Name"
    values = ["${var.environment}-elasticache-sg"]
  }
  vpc_id = var.vpc_id
}

# ── Subnet Group ──────────────────────────────────────────────────────────────
resource "aws_elasticache_subnet_group" "main" {
  name        = "${var.environment}-redis-subnet-group"
  subnet_ids  = data.aws_subnets.private.ids
  description = "Redis subnet group for ${var.environment}"
  tags        = { Name = "${var.environment}-redis-subnet-group" }
}

# ── Parameter Group ───────────────────────────────────────────────────────────
resource "aws_elasticache_parameter_group" "main" {
  name   = "${var.environment}-redis7"
  family = "redis7"

  parameter {
    name  = "maxmemory-policy"
    value = "allkeys-lru"
  }
  parameter {
    name  = "notify-keyspace-events"
    value = "Ex" # keyspace expiry events for cart TTL monitoring
  }

  tags = { Name = "${var.environment}-redis7-params" }
}

# ── Replication Group (pilot: 2 nodes, no cluster mode) ──────────────────────
resource "aws_elasticache_replication_group" "main" {
  replication_group_id = "${var.environment}-zippyra-redis"
  description          = "Zippyra Redis ${var.environment}"

  node_type            = var.node_type
  num_cache_clusters   = var.num_cache_nodes  # 2 for pilot (primary + replica)
  port                 = 6379
  parameter_group_name = aws_elasticache_parameter_group.main.name
  subnet_group_name    = aws_elasticache_subnet_group.main.name
  security_group_ids   = [data.aws_security_group.elasticache.id]

  automatic_failover_enabled = true
  multi_az_enabled           = true

  at_rest_encryption_enabled = true
  transit_encryption_enabled = true
  auth_token                 = var.redis_auth_token

  snapshot_retention_limit = 3
  snapshot_window          = "03:00-04:00"
  maintenance_window       = "sun:04:00-sun:05:00"

  apply_immediately = true

  tags = { Name = "${var.environment}-zippyra-redis" }
}
