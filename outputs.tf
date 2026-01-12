output "s3_bucket_name" {
  value       = module.static_site["corecloud_staticsite_terratest"].s3_bucket_name
}

output "cloudfront_distribution_domain_name" {
  value       = module.static_site["corecloud_staticsite_terratest"].cloudfront_distribution_domain_name
}