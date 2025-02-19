variable "tenant_vars" {
  type = any
}

variable "cloud_front_default_vars" {
  type = any
}

variable "aws_region" {
  type = string
}

variable "web_acl" {
  type = string
}

variable "env" {
  type = string 
}

variable "platform_tags" {
  type = list(string)
  
}

