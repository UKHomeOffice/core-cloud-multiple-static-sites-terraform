variable "tenant_vars" {
  type = map(object({
    repository              = string
    github_environment_name = string
    cost_centre        = string
    account_code       = string
    portfolio_id       = string
    project_id         = string
    service_id         = string
    product            = string
    component          = string
    cloudfront_aliases = list(string)
    cloudfront_cert    = string
   }))
}

variable "cloud_front_default_vars" {
  type = any
}

variable "aws_region" {
  type = string
}

variable "env_name" {
  type = string
}

variable "platform_tags" {
  type = map(string)
}

variable "cloudfront_function_name" {
  type    = string
  default = "StaticSiteReWriteDefaultIndexRequest"
}

