output "vpc_id" {
  value = module.vpc.vpc_id
}

output "eks_cluster_name" {
  value = module.eks.cluster_name
}

output "eks_cluster_endpoint" {
  value = module.eks.cluster_endpoint
}

output "rds_endpoint" {
  value = module.rds.db_endpoint
}

output "rds_proxy_endpoint" {
  value       = module.rds_proxy.proxy_endpoint
  description = "Services connect to proxy, not RDS directly"
}

output "redis_primary_endpoint" {
  value = module.elasticache.redis_primary_endpoint
}

output "kafka_bootstrap_brokers" {
  value     = module.msk.bootstrap_brokers_tls
  sensitive = true
}

output "waf_arn" {
  value = module.waf.waf_arn
}
