output "bootstrap_brokers_tls" {
  value       = aws_msk_cluster.main.bootstrap_brokers_sasl_scram
  description = "TLS bootstrap brokers - use this in all services"
  sensitive   = true
}

output "bootstrap_brokers_plaintext" {
  value       = aws_msk_cluster.main.bootstrap_brokers
  description = "Plaintext bootstrap brokers - local dev only"
}

output "zookeeper_connect_string" {
  value       = aws_msk_cluster.main.zookeeper_connect_string
  description = "Zookeeper connection string"
}

output "cluster_arn" {
  value = aws_msk_cluster.main.arn
}
