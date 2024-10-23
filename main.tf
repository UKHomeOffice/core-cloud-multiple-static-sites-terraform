provider "aws" {
  region = "us-east-1"
  alias  = "us-east-1"
}

module "static_site" {
  source = "git::https://github.com/UKHomeOffice/core-cloud-static-site-terraform.git?ref=CCL-499-c"

  for_each = to_set(var.sites)

  cloud_front_default_vars = var.cloud_front_default_vars
  aws_region               = var.aws_region
  tenant_vars              = var.tenant_vars.each.value

  providers {
    aws.us-east-1 = aws.us-east-1
  }
}
