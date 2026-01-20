output "s3_bucket_name" {
  value       = module.static_site.s3_bucket_name
}

output "cloudfront_distribution_domain_name" {
  value       = module.static_site.cloudfront_distribution_domain_name
}