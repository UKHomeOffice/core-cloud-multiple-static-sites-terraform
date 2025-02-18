variable "tenant_vars" {
  type = object({
    #required for naming of resources
    #e.g. "cc-static-site-${var.tenant_vars.product}-${var.tenant_vars.component}"
    component                       = string
    product                         = string
  })
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
