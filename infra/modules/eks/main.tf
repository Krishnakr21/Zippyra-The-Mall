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

data "aws_security_group" "eks_nodes" {
  filter {
    name   = "tag:Name"
    values = ["${var.environment}-eks-nodes-sg"]
  }
  vpc_id = var.vpc_id
}

data "aws_iam_role" "eks_nodes" {
  name = "${var.environment}-eks-nodes-role"
}

# ── EKS Cluster ───────────────────────────────────────────────────────────────
resource "aws_eks_cluster" "main" {
  name     = "${var.environment}-zippyra"
  role_arn = var.eks_role_arn
  version  = var.cluster_version

  vpc_config {
    subnet_ids              = data.aws_subnets.private.ids
    security_group_ids      = [data.aws_security_group.eks_nodes.id]
    endpoint_private_access = true
    endpoint_public_access  = true # set false after VPN is set up
    public_access_cidrs     = var.public_access_cidrs
  }

  enabled_cluster_log_types = [
    "api", "audit", "authenticator", "controllerManager", "scheduler"
  ]

  tags = { Name = "${var.environment}-zippyra-eks" }
}

# ── OIDC Provider (required for IRSA) ────────────────────────────────────────
data "tls_certificate" "eks" {
  url = aws_eks_cluster.main.identity[0].oidc[0].issuer
}

resource "aws_iam_openid_connect_provider" "eks" {
  client_id_list  = ["sts.amazonaws.com"]
  thumbprint_list = [data.tls_certificate.eks.certificates[0].sha1_fingerprint]
  url             = aws_eks_cluster.main.identity[0].oidc[0].issuer
  tags            = { Name = "${var.environment}-eks-oidc" }
}

# ── EKS Managed Node Group ────────────────────────────────────────────────────
resource "aws_eks_node_group" "main" {
  cluster_name    = aws_eks_cluster.main.name
  node_group_name = "${var.environment}-zippyra-nodes"
  node_role_arn   = data.aws_iam_role.eks_nodes.arn
  subnet_ids      = data.aws_subnets.private.ids
  instance_types  = [var.node_instance_type] # t3.medium for pilot

  scaling_config {
    desired_size = var.node_desired_size # 3
    min_size     = var.node_min_size     # 2
    max_size     = var.node_max_size     # 6
  }

  update_config {
    max_unavailable = 1
  }

  launch_template {
    name    = aws_launch_template.eks_nodes.name
    version = aws_launch_template.eks_nodes.latest_version
  }

  labels = {
    environment = var.environment
    role        = "worker"
  }

  tags = { Name = "${var.environment}-zippyra-nodes" }

  depends_on = [aws_eks_cluster.main]
}

# ── Launch Template (custom userdata, EBS optimized) ─────────────────────────
resource "aws_launch_template" "eks_nodes" {
  name                   = "${var.environment}-eks-node-lt"
  update_default_version = true

  block_device_mappings {
    device_name = "/dev/xvda"
    ebs {
      volume_size           = 50
      volume_type           = "gp3"
      delete_on_termination = true
      encrypted             = true
    }
  }

  monitoring { enabled = true }
  tags = { Name = "${var.environment}-eks-node-lt" }
}

# ── Core Add-ons ──────────────────────────────────────────────────────────────
resource "aws_eks_addon" "coredns" {
  cluster_name                = aws_eks_cluster.main.name
  addon_name                  = "coredns"
  resolve_conflicts_on_create = "OVERWRITE"
  depends_on                  = [aws_eks_node_group.main]
}

resource "aws_eks_addon" "kube_proxy" {
  cluster_name                = aws_eks_cluster.main.name
  addon_name                  = "kube-proxy"
  resolve_conflicts_on_create = "OVERWRITE"
}

resource "aws_eks_addon" "vpc_cni" {
  cluster_name                = aws_eks_cluster.main.name
  addon_name                  = "vpc-cni"
  resolve_conflicts_on_create = "OVERWRITE"
}

resource "aws_eks_addon" "ebs_csi" {
  cluster_name                = aws_eks_cluster.main.name
  addon_name                  = "aws-ebs-csi-driver"
  resolve_conflicts_on_create = "OVERWRITE"
  depends_on                  = [aws_eks_node_group.main]
}

# ── CloudWatch log group for EKS ──────────────────────────────────────────────
resource "aws_cloudwatch_log_group" "eks" {
  name              = "/aws/eks/${var.environment}-zippyra/cluster"
  retention_in_days = 30
  tags              = { Name = "${var.environment}-eks-logs" }
}
