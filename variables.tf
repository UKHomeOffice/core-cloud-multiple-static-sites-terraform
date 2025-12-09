variable "tenant_vars" {
  type = any
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

