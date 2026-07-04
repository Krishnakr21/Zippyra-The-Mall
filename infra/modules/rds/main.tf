data "aws_vpc" "main" {
  id = var.vpc_id
}

data "aws_subnets" "db" {
  filter {
    name   = "vpc-id"
    values = [var.vpc_id]
  }
  filter {
    name   = "tag:Name"
    values = ["${var.environment}-db-*"]
  }
}

data "aws_security_group" "rds" {
  filter {
    name   = "tag:Name"
    values = ["${var.environment}-rds-sg"]
  }
  vpc_id = var.vpc_id
}

# ── DB Subnet Group ───────────────────────────────────────────────────────────
resource "aws_db_subnet_group" "main" {
  name        = "${var.environment}-rds-subnet-group"
  subnet_ids  = data.aws_subnets.db.ids
  description = "RDS subnet group for ${var.environment}"
  tags        = { Name = "${var.environment}-rds-subnet-group" }
}

# ── Parameter Group ───────────────────────────────────────────────────────────
resource "aws_db_parameter_group" "main" {
  name   = "${var.environment}-postgres16"
  family = "postgres16"

  parameter {
    name  = "max_connections"
    value = "200"
  }
  parameter {
    name  = "log_min_duration_statement"
    value = "1000" # log queries taking >1s
  }
  parameter {
    name  = "log_connections"
    value = "1"
  }
  parameter {
    name  = "log_disconnections"
    value = "1"
  }
  parameter {
    name  = "shared_preload_libraries"
    value = "pg_stat_statements"
  }

  tags = { Name = "${var.environment}-postgres16-params" }
}

# ── RDS Instance (Multi-AZ) ───────────────────────────────────────────────────
resource "aws_db_instance" "main" {
  identifier = "${var.environment}-zippyra-postgres"

  engine               = "postgres"
  engine_version       = "16.3"
  instance_class       = var.instance_class
  allocated_storage    = var.allocated_storage
  max_allocated_storage = var.max_allocated_storage
  storage_type         = "gp3"
  storage_encrypted    = true

  db_name  = "zippyra"
  username = "zippyra_admin"
  password = var.db_password

  multi_az               = true
  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [data.aws_security_group.rds.id]
  parameter_group_name   = aws_db_parameter_group.main.name

  backup_retention_period = 7
  backup_window           = "03:00-04:00"
  maintenance_window      = "sun:04:00-sun:05:00"

  deletion_protection     = var.environment == "production" ? true : false
  skip_final_snapshot     = var.environment == "pilot" ? true : false
  final_snapshot_identifier = var.environment == "pilot" ? null : "${var.environment}-zippyra-final-snapshot"

  performance_insights_enabled = true
  monitoring_interval          = 60
  monitoring_role_arn          = aws_iam_role.rds_monitoring.arn

  tags = { Name = "${var.environment}-zippyra-postgres" }
}

# ── Enhanced Monitoring Role ──────────────────────────────────────────────────
resource "aws_iam_role" "rds_monitoring" {
  name = "${var.environment}-rds-monitoring-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "monitoring.rds.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy_attachment" "rds_monitoring" {
  role       = aws_iam_role.rds_monitoring.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonRDSEnhancedMonitoringRole"
}

# ── Read Replica (2 for pilot) ────────────────────────────────────────────────
resource "aws_db_instance" "replica" {
  count = var.replica_count

  identifier          = "${var.environment}-zippyra-postgres-replica-${count.index + 1}"
  replicate_source_db = aws_db_instance.main.identifier
  instance_class      = var.instance_class
  storage_encrypted   = true

  monitoring_interval = 60
  monitoring_role_arn = aws_iam_role.rds_monitoring.arn

  tags = { Name = "${var.environment}-zippyra-postgres-replica-${count.index + 1}" }
}
