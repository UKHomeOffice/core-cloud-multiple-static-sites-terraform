output "s3_bucket_name" {
  value = {
    for key, module in module.static_site :
    key => module.s3_bucket_name
  }
}

output "cloudfront_distribution_domain_name" {
  value = {
    for key, module in module.static_site :
    key => module.cloudfront_distribution_domain_name
  }
}