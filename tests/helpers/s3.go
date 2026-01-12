package helpers

import (
	"bytes"
	"context"
	"log"
	"testing"

	awss3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

func S3Client(t *testing.T) *awss3.Client {
	t.Helper()
	_, config, _ := AWSConfig(t)
	return awss3.NewFromConfig(config)
}

type SeedItem struct {
	Key  string
	HTML []byte
}

// SeedHTMLObjects uploads multiple HTML files to S3
func SeedHTMLObjects(
	t require.TestingT,
	context context.Context,
	s3 *awss3.Client,
	bucket string,
	items []SeedItem,
	region string,
) {
	for _, it := range items {
		_, err := s3.PutObject(context, &awss3.PutObjectInput{
			Bucket:       aws.String(bucket),
			Key:          aws.String(it.Key),
			Body:         bytes.NewReader(it.HTML),
			ContentType:  aws.String("text/html; charset=utf-8"),
			CacheControl: aws.String("no-cache, no-store, must-revalidate"),
		})
		require.NoErrorf(t, err, "failed to upload %s to bucket %q in region %q", it.Key, bucket, region)
	}
}

// CleanUpBucket empties the S3 bucket before destroying the stack.
func CleanUpBucket(t *testing.T) {
	log.Printf("[CLEANUP] Emptying S3 bucket before destroy…")

	context, config, _ := AWSConfig(t)
	s3 := awss3.NewFromConfig(config)

	bucket := TFOutput(t, "s3_bucket_name")
	require.NotEmpty(t, bucket, "bucket name cannot be empty during cleanup")

	paginator := awss3.NewListObjectVersionsPaginator(s3, &awss3.ListObjectVersionsInput{
		Bucket: aws.String(bucket),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context)
		require.NoError(t, err)

		var objectsToDelete []awss3types.ObjectIdentifier

		for _, v := range page.Versions {
			objectsToDelete = append(objectsToDelete, awss3types.ObjectIdentifier{
				Key:       v.Key,
				VersionId: v.VersionId,
			})
		}

		for _, dm := range page.DeleteMarkers {
			objectsToDelete = append(objectsToDelete, awss3types.ObjectIdentifier{
				Key:       dm.Key,
				VersionId: dm.VersionId,
			})
		}

		if len(objectsToDelete) > 0 {
			_, err := s3.DeleteObjects(context, &awss3.DeleteObjectsInput{
				Bucket: aws.String(bucket),
				Delete: &awss3types.Delete{
					Objects: objectsToDelete,
					Quiet:   aws.Bool(true),
				},
			})
			require.NoError(t, err, "failed to delete object versions during cleanup")
		}
	}

	log.Printf("[CLEANUP] Bucket fully emptied: %s", bucket)
}
