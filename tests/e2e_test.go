package test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// AWS SDK v2
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	awss3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func Test_StaticSiteInfra(t *testing.T) {
	t.Parallel()

	_, thisFile, _, _ := runtime.Caller(0)
	thisDir := filepath.Dir(thisFile)
	tfDir := filepath.Clean(filepath.Join(thisDir, ".."))
	varFile := filepath.Clean(filepath.Join(thisDir, "test.tfvars"))

	region := getEnvOrDefault("AWS_REGION", "eu-west-2")
	profile := getEnvOrDefault("AWS_PROFILE", "static-site-test")

	// Build Terraform options (shared across stages)
	tfOpts := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		TerraformDir: tfDir,
		EnvVars: map[string]string{
			"AWS_PROFILE": profile,
			"AWS_REGION":  region,
		},
		VarFiles: []string{varFile},
	})

	// ---------------------------
	// Stage: apply (provision)
	// ---------------------------
	test_structure.RunTestStage(t, "apply", func() {
		terraform.InitAndApply(t, tfOpts)
	})

	// Prepare shared clients/outputs once
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	require.NoError(t, err, "failed to load AWS v2 config")

	// Collect outputs you’ll reuse across subtests
	bucketName := terraform.Output(t, tfOpts, "s3_bucket_name")
	require.NotEmpty(t, bucketName, "terraform output 's3_bucket_name' must not be empty")

	// ---------------------------
	// Stage: validate (assertions)
	// ---------------------------
	test_structure.RunTestStage(t, "validate", func() {

		// Subtest: S3 bucket exists + Public Access Block
		t.Run("Test_S3_Bucket_Security", func(t *testing.T) {
			s3Client := awss3.NewFromConfig(cfg)

			_, err := s3Client.HeadBucket(ctx, &awss3.HeadBucketInput{Bucket: aws.String(bucketName)})
			require.NoErrorf(t, err, "expected bucket %q to exist in region %q", bucketName, region)

			pab, err := s3Client.GetPublicAccessBlock(ctx, &awss3.GetPublicAccessBlockInput{
				Bucket: aws.String(bucketName),
			})
			if err != nil {
				var apiErr smithy.APIError
				if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchPublicAccessBlockConfiguration" {
					require.Failf(t, "public access block missing",
						"S3 bucket %q has NO PublicAccessBlock configuration", bucketName)
				}
				require.NoErrorf(t, err, "failed to get PublicAccessBlock for bucket %q", bucketName)
			}
			cfgPAB := pab.PublicAccessBlockConfiguration
			require.NotNil(t, cfgPAB, "PublicAccessBlockConfiguration should not be nil")

			assert.Truef(t, aws.ToBool(cfgPAB.BlockPublicAcls), "BlockPublicAcls should be true for %q", bucketName)
			assert.Truef(t, aws.ToBool(cfgPAB.IgnorePublicAcls), "IgnorePublicAcls should be true for %q", bucketName)
			assert.Truef(t, aws.ToBool(cfgPAB.BlockPublicPolicy), "BlockPublicPolicy should be true for %q", bucketName)
			assert.Truef(t, aws.ToBool(cfgPAB.RestrictPublicBuckets), "RestrictPublicBuckets should be true for %q", bucketName)
		})

		// Subtest: Versioning status (example)
		t.Run("Test_S3_Bucket_Versioning", func(t *testing.T) {
			s3Client := awss3.NewFromConfig(cfg)
			ver, err := s3Client.GetBucketVersioning(ctx, &awss3.GetBucketVersioningInput{
				Bucket: aws.String(bucketName),
			})
			require.NoErrorf(t, err, "failed to get bucket versioning for %q", bucketName)
			// Expect at least 'Enabled' for production-like environments
			assert.Equal(t, awss3types.BucketVersioningStatusEnabled, ver.Status)
		})
	})

	// ---------------------------
	// Stage: destroy (cleanup)
	// ---------------------------
	test_structure.RunTestStage(t, "destroy", func() {
		terraform.Destroy(t, tfOpts)
	})
}

// small helper
func getEnvOrDefault(key, defVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defVal
}
