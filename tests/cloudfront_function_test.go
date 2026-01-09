package test

import (
	"fmt"
	"log"
	"net/http"
	"testing"
	"time"

	helpers "github.com/core-cloud-multiple-static-sites-terraform/tests/helpers"

	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/gruntwork-io/terratest/modules/terraform"
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	maxAttempts = 30
	delay       = 10 * time.Second
)

func Test_Cloudfront_Function(t *testing.T) {
	t.Parallel()

	tfOpts := helpers.TFOptions(t)

	// Apply stage
	test_structure.RunTestStage(t, "apply", func() {
		log.Printf("[TF] Init+Apply starting…")
		terraform.InitAndApply(t, tfOpts)
		log.Printf("[TF] Init+Apply completed.")
	})

	context, config, region := helpers.AWSConfig(t)
	log.Printf("[AWS] Region: %s", region)

	bucketName := helpers.TFOutput(t, "s3_bucket_name")
	require.NotEmpty(t, bucketName, "terraform output 's3_bucket_name' must not be empty")
	log.Printf("[TF] Output s3_bucket_name: %s", bucketName)

	cfDomain := helpers.TFOutput(t, "cloudfront_distribution_domain_name")
	require.NotEmpty(t, cfDomain, "terraform output 'cloudfront_distribution_domain_name' must not be empty")
	log.Printf("[TF] Output cloudfront_distribution_domain_name: %s", cfDomain)

	// Seed files into the S3 bucket
	test_structure.RunTestStage(t, "seed", func() {
		s3 := awss3.NewFromConfig(config)

		items := []helpers.SeedItem{
			{
				Key:  "index.html",
				HTML: []byte(`<!doctype html><html><head><meta charset="utf-8"><title>Home</title></head><body><h1>Hello from index.html</h1><p>Served from /index.html</p></body></html>`),
			},
			{
				Key:  "about/index.html",
				HTML: []byte(`<!doctype html><html><head><meta charset="utf-8"><title>About</title></head><body><h1>About page index</h1><p>Served from /about/index.html</p></body></html>`),
			},
		}

		for _, it := range items {
			log.Printf("[S3] Seeding object: bucket=%s key=%s content-type=text/html", bucketName, it.Key)
		}
		helpers.SeedHTMLObjects(t, context, s3, bucketName, items, region)
		log.Printf("[S3] Seed complete: %d items written", len(items))
	})

	// Validate stage
	test_structure.RunTestStage(t, "validate", func() {
		baseURL := fmt.Sprintf("https://%s", cfDomain)
		log.Printf("[CF] Base URL: %s", baseURL)

		// Verify that requests to the root path (/) are rewritten to /index.html
		t.Run("Test_CloudFront_Root_Rewrite_To_IndexHTML", func(t *testing.T) {
			urlRoot := baseURL + "/"
			rootResponseStatus, rootResponseBody, rootHeaders := helpers.HttpGetWithRetry(t, urlRoot, maxAttempts, delay, http.StatusOK)
			require.Equal(t, http.StatusOK, rootResponseStatus, "GET / should return 200 from CloudFront")
			assert.Contains(t, rootHeaders.Get("Content-Type"), "text/html", "GET / should return HTML")

			urlIndex := baseURL + "/index.html"
			indexResponseStatus, indexResponseBody, indexHeaders := helpers.HttpGetWithRetry(t, urlIndex, maxAttempts, delay, http.StatusOK)
			require.Equal(t, http.StatusOK, indexResponseStatus, "GET /index.html should return 200 from CloudFront")
			assert.Contains(t, indexHeaders.Get("Content-Type"), "text/html", "GET /index.html should return HTML")

			// Same content for both paths
			assert.Equal(t, indexResponseBody, rootResponseBody,
				"Root path (/) should be rewritten/handled to return the same contents as /index.html")
		})

		// Verify that requests to a subdirectory with trailing slash (/about/) are rewritten to /about/index.html
		t.Run("Test_CloudFront_Subdirectory_TrailingSlash_Rewrite", func(t *testing.T) {
			// GET "/about/"
			urlSlash := baseURL + "/about/"
			slashResponseStatus, slashResponseBody, slashHeaders := helpers.HttpGetWithRetry(t, urlSlash, maxAttempts, delay, http.StatusOK)
			require.Equal(t, http.StatusOK, slashResponseStatus, "GET /about/ should return 200 from CloudFront")
			assert.Contains(t, slashHeaders.Get("Content-Type"), "text/html", "GET /about/ should return HTML")

			// GET "/about/index.html"
			urlIndex := baseURL + "/about/index.html"
			indexResponseStatus, indexResponseBody, indexHeaders := helpers.HttpGetWithRetry(t, urlIndex, maxAttempts, delay, http.StatusOK)
			require.Equal(t, http.StatusOK, indexResponseStatus, "GET /about/index.html should return 200 from CloudFront")
			assert.Contains(t, indexHeaders.Get("Content-Type"), "text/html", "GET /about/index.html should return HTML")

			// Same content for both paths
			assert.Equal(t, indexResponseBody, slashResponseBody,
				"Requests to /about/ should be rewritten/handled to return the same contents as /about/index.html")
		})

		// Verify that requests to a subdirectory without trailing slash (/about) are rewritten to /about/index.html
		t.Run("Test_CloudFront_Subdirectory_NoTrailingSlash_Rewrite", func(t *testing.T) {
			// GET "/about" (no trailing slash)
			urlNoSlash := baseURL + "/about"
			noSlashResponseStatus, noSlashResponseBody, noSlashHeaders := helpers.HttpGetWithRetry(t, urlNoSlash, maxAttempts, delay, http.StatusOK)
			require.Equal(t, http.StatusOK, noSlashResponseStatus, "GET /about should return 200 from CloudFront")
			assert.Contains(t, noSlashHeaders.Get("Content-Type"), "text/html", "GET /about should return HTML")

			// GET "/about/index.html"
			urlIndex := baseURL + "/about/index.html"
			indexResponseStatus, indexResponseBody, indexHeaders := helpers.HttpGetWithRetry(t, urlIndex, maxAttempts, delay, http.StatusOK)
			require.Equal(t, http.StatusOK, indexResponseStatus, "GET /about/index.html should return 200 from CloudFront")
			assert.Contains(t, indexHeaders.Get("Content-Type"), "text/html", "GET /about/index.html should return HTML")

			// Same content for both paths
			assert.Equal(t, indexResponseBody, noSlashResponseBody,
				"Requests to /about (no trailing slash) should be rewritten/handled to return the same contents as /about/index.html")
		})

		// Verify that direct file requests (/about/index.html) are not rewritten
		t.Run("Test_CloudFront_Direct_File_Request_NoRewrite", func(t *testing.T) {
			// GET "/about/index.html" (explicit file request)
			urlIndex := baseURL + "/about/index.html"
			indexResponseStatus, indexResponseBody, indexHeaders := helpers.HttpGetWithRetry(t, urlIndex, maxAttempts, delay, http.StatusOK)

			// Should be served directly (no rewrite / redirect)
			require.Equal(t, http.StatusOK, indexResponseStatus, "GET /about/index.html should return 200 from CloudFront")
			assert.NotContains(t,
				[]int{http.StatusMovedPermanently, http.StatusFound, http.StatusTemporaryRedirect, http.StatusPermanentRedirect},
				indexResponseStatus,
				"Direct file request should not be redirected",
			)
			assert.Contains(t, indexHeaders.Get("Content-Type"), "text/html", "GET /about/index.html should return HTML")

			// Ensure we got the seeded content for about/index.html
			assert.Contains(t, indexResponseBody, "About page index",
				"Direct file request should return contents of about/index.html")
		})

		// Verify that requests to non-existent files (/about/missing.html) are not rewritten and return an error
		t.Run("Test_CloudFront_Nonexistent_Subdirectory_Rewrite_Returns_Error", func(t *testing.T) {
			// 1) GET "/nonexistent/" (trailing slash)
			urlSlash := baseURL + "/nonexistent/"
			slashResponseStatus, bodySlash, hdrSlash := helpers.HttpGetWithRetry(t, urlSlash, maxAttempts, delay, http.StatusNotFound)
			log.Printf("[CF] GET %s -> status=%d content-type=%s length=%d",
				urlSlash, slashResponseStatus, hdrSlash.Get("Content-Type"), len(bodySlash))
			assert.Contains(t, []int{http.StatusForbidden, http.StatusNotFound}, slashResponseStatus,
				"GET /nonexistent/ should return 403 or 404")

			// 2) GET "/nonexistent/index.html" (explicit)
			urlIndex := baseURL + "/nonexistent/index.html"
			indexResponseStatus, indexResponseBody, indexHeaders := helpers.HttpGetWithRetry(t, urlIndex, maxAttempts, delay, http.StatusNotFound)
			log.Printf("[CF] GET %s -> status=%d content-type=%s length=%d",
				urlIndex, indexResponseStatus, indexHeaders.Get("Content-Type"), len(indexResponseBody))
			assert.Contains(t, []int{http.StatusForbidden, http.StatusNotFound}, indexResponseStatus,
				"GET /nonexistent/index.html should return 403 or 404")
		})
	})

	// Destroy stage
	defer test_structure.RunTestStage(t, "destroy", func() {
		helpers.CleanUpBucket(t)
		log.Printf("[TF] Destroy starting…")
		terraform.Destroy(t, tfOpts)
		log.Printf("[TF] Destroy completed.")
	})
}
