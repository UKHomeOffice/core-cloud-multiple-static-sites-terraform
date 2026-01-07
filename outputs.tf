output "s3_bucket_name" {
  description = "Bubble up the bucket name from the module instance"
  value       = module.static_site["corecloud_staticsite_terratest"].s3_bucket_name
}

output "cloudfront_distribution_domain_name" {
  description = "Bubble up the CloudFront distribution domain name from the module instance"
  value       = module.static_site["corecloud_staticsite_terratest"].cloudfront_distribution_domain_name
}