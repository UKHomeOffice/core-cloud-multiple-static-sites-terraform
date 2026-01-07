package test

import (
	"errors"
	"log"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	awss3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	helpers "github.com/core-cloud-multiple-static-sites-terraform/tests/helpers"
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"
)

// logAPIError expands smithy API errors with useful details for triage.
func logAPIError(t *testing.T, err error) {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		t.Logf("[AWS ERROR] code=%s message=%s", apiErr.ErrorCode(), apiErr.Error())
		type statusCarrier interface{ HTTPStatusCode() int }
		if sc, ok := apiErr.(statusCarrier); ok {
			t.Logf("[AWS ERROR] http_status=%d", sc.HTTPStatusCode())
		}
	}
}

func Test_S3(t *testing.T) {
	t.Parallel()

	// ---------------------------
	// Build TF options once
	// ---------------------------
	tfOpts := helpers.TFOptions(t)
	log.Printf("[TF] Using working dir: %s", tfOpts.TerraformDir)

	// ---------------------------
	// Stage: apply
	// ---------------------------
	test_structure.RunTestStage(t, "apply", func() {
		log.Printf("[TF] Init+Apply starting…")
		terraform.InitAndApply(t, tfOpts)
		log.Printf("[TF] Init+Apply completed.")
	})

	// Prepare shared clients/outputs once
	ctx, _, region := helpers.AWSConfig(t)
	log.Printf("[AWS] Region resolved: %s", region)

	bucketName := helpers.TFOutput(t, "s3_bucket_name")
	require.NotEmpty(t, bucketName, "terraform output 's3_bucket_name' must not be empty")
	log.Printf("[TF] Output s3_bucket_name: %s", bucketName)

	// ---------------------------
	// Stage: validate
	// ---------------------------
	test_structure.RunTestStage(t, "validate", func() {
		s3 := helpers.S3Client(t)
		t.Logf("[S3] Client created for region %s", region)

		t.Run("Test_S3_Bucket_Security", func(t *testing.T) {
			// HeadBucket
			t.Logf("[S3] HeadBucket: bucket=%s region=%s", bucketName, region)
			_, err := s3.HeadBucket(ctx, &awss3.HeadBucketInput{Bucket: aws.String(bucketName)})
			if err != nil {
				logAPIError(t, err)
			}
			require.NoErrorf(t, err, "expected bucket %q to exist in region %q", bucketName, region)
			t.Logf("[S3] HeadBucket OK: bucket=%s", bucketName)

			// Public Access Block
			t.Logf("[S3] GetPublicAccessBlock: bucket=%s", bucketName)
			pab, err := s3.GetPublicAccessBlock(ctx, &awss3.GetPublicAccessBlockInput{
				Bucket: aws.String(bucketName),
			})
			if err != nil {
				logAPIError(t, err)
				var apiErr smithy.APIError
				if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchPublicAccessBlockConfiguration" {
					require.Failf(t, "public access block missing",
						"S3 bucket %q has NO PublicAccessBlock configuration", bucketName)
				}
				require.NoErrorf(t, err, "failed to get PublicAccessBlock for bucket %q", bucketName)
			}
			t.Logf("[S3] GetPublicAccessBlock OK: bucket=%s", bucketName)

			cfgPAB := pab.PublicAccessBlockConfiguration
			require.NotNil(t, cfgPAB, "PublicAccessBlockConfiguration should not be nil")

			// Log full PAB values for quick triage
			t.Logf("[S3] PAB values: BlockPublicAcls=%t IgnorePublicAcls=%t BlockPublicPolicy=%t RestrictPublicBuckets=%t",
				aws.ToBool(cfgPAB.BlockPublicAcls),
				aws.ToBool(cfgPAB.IgnorePublicAcls),
				aws.ToBool(cfgPAB.BlockPublicPolicy),
				aws.ToBool(cfgPAB.RestrictPublicBuckets),
			)

			assert.Truef(t, aws.ToBool(cfgPAB.BlockPublicAcls), "BlockPublicAcls should be true for %q", bucketName)
			assert.Truef(t, aws.ToBool(cfgPAB.IgnorePublicAcls), "IgnorePublicAcls should be true for %q", bucketName)
			assert.Truef(t, aws.ToBool(cfgPAB.BlockPublicPolicy), "BlockPublicPolicy should be true for %q", bucketName)
			assert.Truef(t, aws.ToBool(cfgPAB.RestrictPublicBuckets), "RestrictPublicBuckets should be true for %q", bucketName)
		})

		t.Run("Test_S3_Bucket_Versioning", func(t *testing.T) {
			t.Logf("[S3] GetBucketVersioning: bucket=%s region=%s", bucketName, region)
			ver, err := s3.GetBucketVersioning(ctx, &awss3.GetBucketVersioningInput{
				Bucket: aws.String(bucketName),
			})
			if err != nil {
				logAPIError(t, err)
			}
			require.NoErrorf(t, err, "failed to get bucket versioning for %q", bucketName)

			t.Logf("[S3] Versioning response: Status=%s MFADelete=%s",
				ver.Status, ver.MFADelete)

			assert.Equal(t, awss3types.BucketVersioningStatusEnabled, ver.Status, "versioning must be enabled")
		})
	})

	// ---------------------------
	// Stage: destroy
	// ---------------------------
	defer test_structure.RunTestStage(t, "destroy", func() {
		terraform.Destroy(t, tfOpts)
	})
}
