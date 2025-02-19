provider "aws" {
  region = "us-east-1"
  alias  = "us-east-1"
}

module "waf" {
  source            = "git::https://github.com/UKHomeOffice/core-cloud-static-sites-wafv2-terraform.git?ref=0.4.6"
  waf_acl_name      = "cc-static-site-${var.env_name}-acl"
  tags              = var.platform_tags
  scope             = "CLOUDFRONT"
}

module "static_site" {
  source = "git::https://github.com/UKHomeOffice/core-cloud-static-site-terraform.git?ref=0.2.1"

  for_each = var.tenant_vars

  cloud_front_default_vars = var.cloud_front_default_vars
  aws_region               = var.aws_region
  tenant_vars              = each.value
  waf_acl_id               = module.waf.waf_acl_id
  providers = {
    aws.us-east-1 = aws.us-east-1
  }
}
