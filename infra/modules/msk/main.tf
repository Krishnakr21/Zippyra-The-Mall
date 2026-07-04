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

data "aws_security_group" "msk" {
  filter {
    name   = "tag:Name"
    values = ["${var.environment}-msk-sg"]
  }
  vpc_id = var.vpc_id
}

# ── MSK Cluster Configuration ─────────────────────────────────────────────────
resource "aws_msk_configuration" "main" {
  name              = "${var.environment}-zippyra-kafka-config"
  kafka_versions    = ["3.5.1"]
  description       = "Zippyra Kafka config - ${var.environment}"

  server_properties = <<-EOT
    auto.create.topics.enable=false
    default.replication.factor=3
    min.insync.replicas=2
    num.partitions=3
    log.retention.hours=168
    log.segment.bytes=1073741824
    log.retention.check.interval.ms=300000
    num.recovery.threads.per.data.dir=1
    offsets.topic.replication.factor=3
    transaction.state.log.min.isr=2
    transaction.state.log.replication.factor=3
  EOT
}

# ── MSK Cluster ───────────────────────────────────────────────────────────────
resource "aws_msk_cluster" "main" {
  cluster_name           = "${var.environment}-zippyra-kafka"
  kafka_version          = "3.5.1"
  number_of_broker_nodes = var.number_of_broker_nodes # 3 for pilot

  broker_node_group_info {
    instance_type  = var.broker_instance_type # kafka.t3.small for pilot
    client_subnets = slice(tolist(data.aws_subnets.private.ids), 0, var.number_of_broker_nodes)

    storage_info {
      ebs_storage_info {
        volume_size = var.broker_volume_size # 100 GB per broker
      }
    }

    security_groups = [data.aws_security_group.msk.id]
  }

  configuration_info {
    arn      = aws_msk_configuration.main.arn
    revision = aws_msk_configuration.main.latest_revision
  }

  encryption_info {
    encryption_in_transit {
      client_broker = "TLS_PLAINTEXT"
      in_cluster    = true
    }
  }

  client_authentication {
    sasl {
      scram = true
    }
  }

  open_monitoring {
    prometheus {
      jmx_exporter {
        enabled_in_broker = true
      }
      node_exporter {
        enabled_in_broker = true
      }
    }
  }

  logging_info {
    broker_logs {
      cloudwatch_logs {
        enabled   = true
        log_group = "/aws/msk/${var.environment}-zippyra-kafka"
      }
    }
  }

  tags = { Name = "${var.environment}-zippyra-kafka" }
}

# ── CloudWatch log group for MSK ──────────────────────────────────────────────
resource "aws_cloudwatch_log_group" "msk" {
  name              = "/aws/msk/${var.environment}-zippyra-kafka"
  retention_in_days = 30
  tags              = { Name = "${var.environment}-msk-logs" }
}
