provider "aws" {
  region = "us-east-1"
  alias  = "us-east-1"
}

module "waf" {
  source            = "git::https://github.com/UKHomeOffice/core-cloud-static-sites-wafv2-terraform.git?ref=0.4.4"
  waf_acl_name      = "cc-${var.tenant_vars.product}-${var.tenant_vars.component}"
  tags              = {
    Environment = "test"
    Project     = "Static Sites"
  }
}

module "static_site" {
  source = "git::https://github.com/UKHomeOffice/core-cloud-static-site-terraform.git?ref=0.1.5"

  for_each = var.tenant_vars

  cloud_front_default_vars = var.cloud_front_default_vars
  aws_region               = var.aws_region
  tenant_vars              = each.value
  web_acl_id               = var.waf_arn.waf_acl_id
  providers = {
    aws.us-east-1 = aws.us-east-1
  }
}
