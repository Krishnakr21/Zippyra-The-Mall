output "eks_role_arn" {
  value       = aws_iam_role.eks_cluster.arn
  description = "EKS cluster role ARN - consumed by eks module"
}

output "eks_nodes_role_arn" {
  value       = aws_iam_role.eks_nodes.arn
  description = "EKS node group role ARN"
}

output "kms_key_arn" {
  value       = aws_kms_key.main.arn
  description = "KMS key ARN for secrets encryption"
}

output "kms_key_id" {
  value       = aws_kms_key.main.key_id
  description = "KMS key ID"
}
