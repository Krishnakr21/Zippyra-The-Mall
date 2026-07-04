data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

# ── KMS Key for secrets encryption ───────────────────────────────────────────
resource "aws_kms_key" "main" {
  description             = "${var.environment} - Zippyra secrets encryption"
  deletion_window_in_days = 7
  enable_key_rotation     = true
  tags                    = { Name = "${var.environment}-kms" }
}

resource "aws_kms_alias" "main" {
  name          = "alias/${var.environment}-zippyra"
  target_key_id = aws_kms_key.main.key_id
}

# ── EKS Cluster Role ──────────────────────────────────────────────────────────
resource "aws_iam_role" "eks_cluster" {
  name = "${var.environment}-eks-cluster-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "eks.amazonaws.com" }
    }]
  })
  tags = { Name = "${var.environment}-eks-cluster-role" }
}

resource "aws_iam_role_policy_attachment" "eks_cluster_policy" {
  role       = aws_iam_role.eks_cluster.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
}

# ── EKS Node Group Role ───────────────────────────────────────────────────────
resource "aws_iam_role" "eks_nodes" {
  name = "${var.environment}-eks-nodes-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "ec2.amazonaws.com" }
    }]
  })
  tags = { Name = "${var.environment}-eks-nodes-role" }
}

resource "aws_iam_role_policy_attachment" "eks_worker_node" {
  role       = aws_iam_role.eks_nodes.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"
}

resource "aws_iam_role_policy_attachment" "eks_cni" {
  role       = aws_iam_role.eks_nodes.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"
}

resource "aws_iam_role_policy_attachment" "eks_ecr_readonly" {
  role       = aws_iam_role.eks_nodes.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
}

resource "aws_iam_role_policy_attachment" "eks_ssm" {
  role       = aws_iam_role.eks_nodes.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

# ── Inline policy: nodes can read Secrets Manager ─────────────────────────────
resource "aws_iam_role_policy" "eks_nodes_secrets" {
  name = "${var.environment}-eks-nodes-secrets"
  role = aws_iam_role.eks_nodes.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "secretsmanager:GetSecretValue",
          "secretsmanager:DescribeSecret"
        ]
        Resource = "arn:aws:secretsmanager:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:secret:${var.environment}/zippyra/*"
      },
      {
        Effect   = "Allow"
        Action   = ["kms:Decrypt", "kms:GenerateDataKey"]
        Resource = aws_kms_key.main.arn
      }
    ]
  })
}

# ── Inline policy: nodes can write to S3 invoices/media ──────────────────────
resource "aws_iam_role_policy" "eks_nodes_s3" {
  name = "${var.environment}-eks-nodes-s3"
  role = aws_iam_role.eks_nodes.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = [
        "s3:GetObject",
        "s3:PutObject",
        "s3:DeleteObject",
        "s3:GetObjectVersion"
      ]
      Resource = [
        "arn:aws:s3:::${var.environment}-zippyra-invoices/*",
        "arn:aws:s3:::${var.environment}-zippyra-products/*",
        "arn:aws:s3:::${var.environment}-zippyra-media/*"
      ]
    }]
  })
}
