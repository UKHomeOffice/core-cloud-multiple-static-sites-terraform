resource "aws_cloudfront_function" "rewritedefaultindexrequest" {
  name    = "StaticSiteReWriteDefaultIndexRequest-${var.component}"
  runtime = "cloudfront-js-2.0"
  publish = true
  code    = file("${path.module}/resources/rewriteindex.js")
}
