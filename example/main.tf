module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "5.0.0"

  name = "example-vpc"
  cidr = "10.0.0.0/16"
}

module "eks" {
  source = "git::https://github.com/terraform-aws-modules/terraform-aws-eks.git?ref=v19.0.0"

  cluster_name = "example-cluster"
}

module "rds" {
  source  = "terraform-aws-modules/rds/aws"
  version = "6.1.0"

  identifier = "example-db"
}
