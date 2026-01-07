package helpers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

func AWSConfig(t *testing.T) (context.Context, aws.Config, string) {
	t.Helper()
	onceAWS.Do(func() {
		awsCtx = context.Background()
		cfg, err := config.LoadDefaultConfig(awsCtx, config.WithRegion(awsRegion))
		require.NoError(t, err)
		awsCfg = cfg
	})
	return awsCtx, awsCfg, awsRegion
}

// Example client builder
func S3Client(t *testing.T) *awss3.Client {
	t.Helper()
	_, cfg, _ := AWSConfig(t)
	return awss3.NewFromConfig(cfg)
}

// httpGetWithRetry performs a GET with retries to allow for CloudFront propagation.
// Returns status code, body string, and headers.
func HttpGetWithRetry(t *testing.T, url string, attempts int, delay time.Duration) (int, string, http.Header) {
	t.Helper()

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		resp, err := client.Get(url)
		if err != nil {
			lastErr = err
			time.Sleep(delay)
			continue
		}

		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		body := string(bodyBytes)

		// Consider success on 200 with non-empty body
		if resp.StatusCode == http.StatusOK && len(body) > 0 {
			return resp.StatusCode, body, resp.Header
		}

		// Retry on non-200 or empty bodies to allow origin/CF readiness
		lastErr = fmt.Errorf("status=%d, body_len=%d", resp.StatusCode, len(body))
		time.Sleep(delay)
	}

	require.Failf(t, "http retries exhausted",
		"GET %s failed after %d attempts: last error: %v", url, attempts, lastErr)
	return 0, "", nil
}

type SeedItem struct {
	Key  string
	HTML []byte
}

// SeedHTMLObjects uploads multiple HTML files to S3 with proper headers.
// Cache-Control disables caching to avoid stale edge content during tests.
func SeedHTMLObjects(
	t require.TestingT,
	ctx context.Context,
	s3 *awss3.Client,
	bucket string,
	items []SeedItem,
	region string,
) {
	for _, it := range items {
		_, err := s3.PutObject(ctx, &awss3.PutObjectInput{
			Bucket:       aws.String(bucket),
			Key:          aws.String(it.Key),
			Body:         bytes.NewReader(it.HTML),
			ContentType:  aws.String("text/html; charset=utf-8"),
			CacheControl: aws.String("no-cache, no-store, must-revalidate"),
		})
		require.NoErrorf(t, err, "failed to upload %s to bucket %q in region %q", it.Key, bucket, region)
	}
}
