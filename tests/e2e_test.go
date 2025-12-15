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
)

func Test_S3Bucket(t *testing.T) {
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

	// ---------------------------
	// Stage: validate (assertions)
	// ---------------------------
	test_structure.RunTestStage(t, "validate", func() {
		// Read outputs
		bucketName := terraform.Output(t, tfOpts, "bucket_name")
		require.NotEmpty(t, bucketName, "terraform output 'bucket_name' must not be empty")

		// AWS v2 client
		ctx := context.Background()
		cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
		require.NoError(t, err, "failed to load AWS v2 config")
		s3Client := awss3.NewFromConfig(cfg)

		// Exists
		_, err = s3Client.HeadBucket(ctx, &awss3.HeadBucketInput{Bucket: aws.String(bucketName)})
		require.NoErrorf(t, err, "expected bucket %q to exist in region %q", bucketName, region)

		// Public Access Block
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
