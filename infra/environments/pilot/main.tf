terraform {
  required_version = ">= 1.5.0"
  required_providers {
    aws = { source = "hashicorp/aws", version = "~> 6.0" }
    tls = { source = "hashicorp/tls", version = "~> 4.0" }
  }
}

provider "aws" {
  region = var.aws_region
  default_tags {
    tags = {
      Environment = "pilot"
      Project     = "zippyra"
      ManagedBy   = "terraform"
    }
  }
}

locals {
  environment = "pilot"
  vpc_cidr    = "10.10.0.0/16"
}

module "vpc" {
  source      = "../../modules/vpc"
  environment = local.environment
  vpc_cidr    = local.vpc_cidr
}

module "iam" {
  source      = "../../modules/iam"
  environment = local.environment
  vpc_id      = module.vpc.vpc_id
  depends_on  = [module.vpc]
}

module "rds" {
  source                = "../../modules/rds"
  environment           = local.environment
  vpc_id                = module.vpc.vpc_id
  db_password           = var.db_password
  instance_class        = "db.t4g.large"
  allocated_storage     = 100
  max_allocated_storage = 500
  replica_count         = 2
  depends_on            = [module.vpc, module.iam]
}

module "rds_proxy" {
  source                 = "../../modules/rds_proxy"
  environment            = local.environment
  vpc_id                 = module.vpc.vpc_id
  private_subnet_ids     = module.vpc.private_subnet_ids
  eks_nodes_sg_ids       = [module.vpc.eks_nodes_sg_id]
  db_instance_identifier = module.rds.db_instance_id
  db_secret_arn          = module.secrets.db_secret_arn
  kms_key_arn            = module.iam.kms_key_arn
  depends_on             = [module.rds]
}

module "elasticache" {
  source           = "../../modules/elasticache"
  environment      = local.environment
  vpc_id           = module.vpc.vpc_id
  redis_auth_token = var.redis_auth_token
  node_type        = "cache.t4g.medium"
  num_cache_nodes  = 2
  depends_on       = [module.vpc, module.iam]
}

module "msk" {
  source                 = "../../modules/msk"
  environment            = local.environment
  vpc_id                 = module.vpc.vpc_id
  number_of_broker_nodes = 3
  broker_instance_type   = "kafka.t3.small"
  broker_volume_size     = 100
  depends_on             = [module.rds, module.elasticache]
}

module "eks" {
  source              = "../../modules/eks"
  environment         = local.environment
  vpc_id              = module.vpc.vpc_id
  eks_role_arn        = module.iam.eks_role_arn
  cluster_version     = "1.29"
  node_instance_type  = "t3.medium"
  node_desired_size   = 3
  node_min_size       = 2
  node_max_size       = 6
  public_access_cidrs = var.allowed_cidr_blocks
  depends_on          = [module.msk]
}

module "waf" {
  source      = "../../modules/waf"
  environment = local.environment
  alb_arn     = var.alb_arn
  depends_on  = [module.eks]
}

module "secrets" {
  source           = "../../modules/secrets"
  environment      = local.environment
  db_password      = var.db_password
  redis_auth_token = var.redis_auth_token
  kms_key_arn      = module.iam.kms_key_arn
}

module "s3" {
  source      = "../../modules/s3"
  environment = local.environment
  kms_key_arn = module.iam.kms_key_arn
}

module "cloudfront" {
  source                 = "../../modules/cloudfront"
  environment            = local.environment
  products_bucket_id     = module.s3.products_bucket_id
  products_bucket_domain = module.s3.products_bucket_domain
}
