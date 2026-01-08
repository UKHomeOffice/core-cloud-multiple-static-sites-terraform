package helpers

import (
	"bytes"
	"context"
	"testing"

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

// SeedHTMLObjects uploads multiple HTML files to S3 with proper headers.
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
