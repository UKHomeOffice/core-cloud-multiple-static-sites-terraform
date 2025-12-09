resource "aws_cloudfront_function" "rewritedefaultindexrequest" {
  name    = "${var.cloudfront_function_name}"
  runtime = "cloudfront-js-2.0"
  publish = true
  code    = file("${path.module}/resources/rewriteindex.js")
}
