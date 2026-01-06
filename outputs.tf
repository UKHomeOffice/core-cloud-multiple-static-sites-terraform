output "s3_bucket_name" {
  description = "Bubble up the bucket name from the module instance"
  value       = module.static_site["corecloud_staticsite_terratest"].s3_bucket_name
}
