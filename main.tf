provider "aws" {
  region = "us-east-1"
  alias  = "us-east-1"
}

module "static_site" {
  source = "git::https://github.com/UKHomeOffice/core-cloud-static-site-terraform.git?ref=0.0.2"

  for_each = var.tenant_vars

  cloud_front_default_vars = var.cloud_front_default_vars
  aws_region               = var.aws_region
  tenant_vars              = each.value

  providers = {
    aws.us-east-1 = aws.us-east-1
  }
}
