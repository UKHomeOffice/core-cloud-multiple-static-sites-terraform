package test

import (
	"context"
	"errors"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// AWS SDK v2
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

func TestS3Bucket_ExistsAndPublicAccessBlocked(t *testing.T) {
	t.Parallel()

	// --- Terraform options ---
	tfOpts := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		TerraformDir: "../",
		EnvVars: map[string]string{
			"AWS_PROFILE": "static-site-test",
			"AWS_REGION":  "eu-west-2",
		},
		VarFiles: []string{"test.tfvars"},
	})

	// Apply once, destroy at the end
	defer terraform.Destroy(t, tfOpts)
	terraform.InitAndApply(t, tfOpts)

	// --- Read outputs ---
	region := tfOpts.EnvVars["AWS_REGION"]
	bucketName := terraform.Output(t, tfOpts, "bucket_name")
	require.NotEmpty(t, bucketName, "terraform output 'bucket_name' must not be empty")

	// --- AWS SDK v2 client ---
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	require.NoError(t, err, "failed to load AWS v2 config")

	s3Client := awss3.NewFromConfig(cfg)

	// --- Assert bucket exists (HeadBucket) ---
	_, err = s3Client.HeadBucket(ctx, &awss3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})
	require.NoErrorf(t, err, "expected bucket %q to exist in region %q", bucketName, region)

	// --- Assert Public Access Block fully enabled ---
	pab, err := s3Client.GetPublicAccessBlock(ctx, &awss3.GetPublicAccessBlockInput{
		Bucket: aws.String(bucketName),
	})

	if err != nil {
		// Use smithy.APIError to check modeled error codes safely across SDK versions
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchPublicAccessBlockConfiguration" {
			require.Failf(t, "public access block missing",
				"S3 bucket %q has NO PublicAccessBlock configuration", bucketName)
		}
		require.NoErrorf(t, err, "failed to get PublicAccessBlock for bucket %q", bucketName)
	}

	cfgPAB := pab.PublicAccessBlockConfiguration
	require.NotNil(t, cfgPAB, "PublicAccessBlockConfiguration should not be nil")

	assert.Truef(t, aws.ToBool(cfgPAB.BlockPublicAcls), "BlockPublicAcls should be true for bucket %q", bucketName)
	assert.Truef(t, aws.ToBool(cfgPAB.IgnorePublicAcls), "IgnorePublicAcls should be true for bucket %q", bucketName)
	assert.Truef(t, aws.ToBool(cfgPAB.BlockPublicPolicy), "BlockPublicPolicy should be true for bucket %q", bucketName)
	assert.Truef(t, aws.ToBool(cfgPAB.RestrictPublicBuckets), "RestrictPublicBuckets should be true for bucket %q", bucketName)
}
