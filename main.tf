provider "aws" {
  region = "us-east-1"
  alias  = "us-east-1"
}

module "waf" {
  source       = "git::https://github.com/UKHomeOffice/core-cloud-static-sites-wafv2-terraform.git?ref=0.4.6"
  
  for_each     = var.tenant_vars

  waf_acl_name = "cc-static-site-${var.env_name}-${each.value.component}-acl"
  tags         = var.platform_tags
  scope        = "CLOUDFRONT"
}

module "cloudfront" {
  for_each     = var.tenant_vars
    
  source = "./cloudfront-function-terraform/"
  component = each.value.component
}

module "static_site" {
  source = "git::https://github.com/UKHomeOffice/core-cloud-static-site-terraform.git?ref=0.3.1"

  for_each = var.tenant_vars

  cloudfront_function_rewrite_arn = module.cloudfront.cloudfront_function_rewritedefaultindexrequest_arn
  cloud_front_default_vars        = var.cloud_front_default_vars
  aws_region                      = var.aws_region
  tenant_vars                     = each.value
  waf_acl_id                      = module.waf.waf_acl_arn # cloudfront_distribution input variable waf_acl_id is actually the arn
  providers = {
    aws.us-east-1 = aws.us-east-1
  }
}
