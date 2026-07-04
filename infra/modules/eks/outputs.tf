output "cluster_name" {
  value       = aws_eks_cluster.main.name
  description = "EKS cluster name"
}

output "cluster_endpoint" {
  value       = aws_eks_cluster.main.endpoint
  description = "EKS API server endpoint"
}

output "cluster_ca" {
  value       = aws_eks_cluster.main.certificate_authority[0].data
  description = "EKS cluster CA certificate"
  sensitive   = true
}

output "oidc_provider_arn" {
  value       = aws_iam_openid_connect_provider.eks.arn
  description = "OIDC provider ARN for IRSA"
}

output "oidc_provider_url" {
  value       = aws_iam_openid_connect_provider.eks.url
  description = "OIDC provider URL for IRSA"
}
