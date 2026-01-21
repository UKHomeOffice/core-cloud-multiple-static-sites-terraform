# Terratest — CloudFront Function tests

Purpose
- Integration tests that provision the Terraform stack, seed an S3 bucket, and validate CloudFront request-rewrite behavior implemented by the CloudFront Function.
- Test entrypoint: [cloudfront_function_test.go](./cloudfront_function_test.go)

Files
- Tests: [cloudfront_function_test.go](./cloudfront_function_test.go)  
- Helpers: [helpers/common.go](./helpers/common.go), [helpers/s3.go](./helpers/s3.go)  
- Optional test var file: [helpers/test.tfvars](./helpers/test.tfvars)  
- Go modules: [go.mod](./go.mod)

Related repo files (relative links from this folder)
- Terraform module: [../cloudfront-function-terraform/main.tf](../cloudfront-function-terraform/main.tf)  
- CI workflow: [../.github/workflows/run-terratest.yml](../.github/workflows/run-terratest.yml)

Quick overview
- The tests use Terratest + Go to run `terraform init` / `apply`, seed S3 objects (`index.html`, `about/index.html`), make HTTP requests against the CloudFront distribution, assert rewrite behavior, then `terraform destroy`.
- Helpers provide TF options, AWS config, HTTP retry logic, S3 seeding, and bucket cleanup.

Prerequisites
- Go (CI uses 1.22)
- Terraform (CI uses 1.9.8)
- AWS credentials / ability to assume the role used in CI
- Optional: populate `helpers/test.tfvars` or set TF_VAR_* env vars used by `helpers.TFOptions`

Run locally
1. Download modules:
   - cd tests && go mod download
2. Authenicate locally with AWS and update the value in `helpers.TFOptions`
3. Run tests:
   - cd tests && go test -v -timeout 45m

CI
- The repository workflow that runs these tests is at `.github/workflows/run-terratest.yml`. It fetches modules, configures AWS credentials, and runs `go test` in the `tests/` directory.