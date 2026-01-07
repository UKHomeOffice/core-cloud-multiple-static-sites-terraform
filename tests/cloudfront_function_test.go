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

func Test_Cloudfront_Function(t *testing.T) {
	t.Parallel()

	// Build TF opts once
	tfOpts := helpers.TFOptions(t)
	log.Printf("[TF] Working dir: %s", tfOpts.TerraformDir)
	if len(tfOpts.Vars) > 0 {
		log.Printf("[TF] Vars: %#v", tfOpts.Vars)
	}

	// Explicitly log the active workspace (Terratest Options doesn't track it)
	ws := terraform.RunTerraformCommand(t, tfOpts, "workspace", "show")
	log.Printf("[TF] Workspace: %s", ws)

	// ---------------------------
	// Stage: apply
	// ---------------------------
	test_structure.RunTestStage(t, "apply", func() {
		log.Printf("[TF] Init+Apply starting…")
		terraform.InitAndApply(t, tfOpts)
		log.Printf("[TF] Init+Apply completed.")
	})

	// Shared clients/outputs
	ctx, cfg, region := helpers.AWSConfig(t)
	log.Printf("[AWS] Region resolved: %s", region)

	bucketName := helpers.TFOutput(t, "s3_bucket_name")
	require.NotEmpty(t, bucketName, "terraform output 's3_bucket_name' must not be empty")
	log.Printf("[TF] Output s3_bucket_name: %s", bucketName)

	cfDomain := helpers.TFOutput(t, "cloudfront_distribution_domain_name")
	require.NotEmpty(t, cfDomain, "terraform output 'cloudfront_distribution_domain_name' must not be empty")
	log.Printf("[TF] Output cloudfront_distribution_domain_name: %s", cfDomain)

	// ---------------------------
	// Stage: seed origin (S3) via batch helper
	// ---------------------------
	test_structure.RunTestStage(t, "seed", func() {
		s3 := awss3.NewFromConfig(cfg)

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
		helpers.SeedHTMLObjects(t, ctx, s3, bucketName, items, region)
		log.Printf("[S3] Seed complete: %d items written", len(items))
	})

	// ---------------------------
	// Stage: validate CloudFront behavior
	// ---------------------------
	test_structure.RunTestStage(t, "validate", func() {
		baseURL := fmt.Sprintf("https://%s", cfDomain)
		log.Printf("[CF] Base URL: %s", baseURL)

		// Case 1: Root path "/" should return same content as "/index.html"
		t.Run("Test_CloudFront_Root_Rewrite_To_IndexHTML", func(t *testing.T) {
			const (
				maxAttempts = 30
				delay       = 10 * time.Second
			)
			log.Printf("[CF] Starting retry loop (root): maxAttempts=%d delay=%s", maxAttempts, delay)

			// GET "/"
			urlRoot := baseURL + "/"
			statusRoot, bodyRoot, hdrRoot := helpers.HttpGetWithRetry(t, urlRoot, maxAttempts, delay)
			log.Printf("[CF] GET %s -> status=%d content-type=%s length=%d",
				urlRoot, statusRoot, hdrRoot.Get("Content-Type"), len(bodyRoot))
			require.Equal(t, http.StatusOK, statusRoot, "GET / should return 200 from CloudFront")
			assert.Contains(t, hdrRoot.Get("Content-Type"), "text/html", "GET / should return HTML")

			// GET "/index.html"
			urlIndex := baseURL + "/index.html"
			statusIndex, bodyIndex, hdrIndex := helpers.HttpGetWithRetry(t, urlIndex, maxAttempts, delay)
			log.Printf("[CF] GET %s -> status=%d content-type=%s length=%d",
				urlIndex, statusIndex, hdrIndex.Get("Content-Type"), len(bodyIndex))
			require.Equal(t, http.StatusOK, statusIndex, "GET /index.html should return 200 from CloudFront")
			assert.Contains(t, hdrIndex.Get("Content-Type"), "text/html", "GET /index.html should return HTML")

			// Same content for both paths
			assert.Equal(t, bodyIndex, bodyRoot,
				"Root path (/) should be rewritten/handled to return the same contents as /index.html")
		})

		// Case 2: Subdirectory "/about/" should return same content as "/about/index.html"
		t.Run("Test_CloudFront_Subdirectory_TrailingSlash_Rewrite", func(t *testing.T) {
			const (
				maxAttempts = 30
				delay       = 10 * time.Second
			)
			log.Printf("[CF] Starting retry loop (subdir): maxAttempts=%d delay=%s", maxAttempts, delay)

			// GET "/about/"
			urlSlash := baseURL + "/about/"
			statusSlash, bodySlash, hdrSlash := helpers.HttpGetWithRetry(t, urlSlash, maxAttempts, delay)
			log.Printf("[CF] GET %s -> status=%d content-type=%s length=%d",
				urlSlash, statusSlash, hdrSlash.Get("Content-Type"), len(bodySlash))
			require.Equal(t, http.StatusOK, statusSlash, "GET /about/ should return 200 from CloudFront")
			assert.Contains(t, hdrSlash.Get("Content-Type"), "text/html", "GET /about/ should return HTML")

			// GET "/about/index.html"
			urlIndex := baseURL + "/about/index.html"
			statusIndex, bodyIndex, hdrIndex := helpers.HttpGetWithRetry(t, urlIndex, maxAttempts, delay)
			log.Printf("[CF] GET %s -> status=%d content-type=%s length=%d",
				urlIndex, statusIndex, hdrIndex.Get("Content-Type"), len(bodyIndex))
			require.Equal(t, http.StatusOK, statusIndex, "GET /about/index.html should return 200 from CloudFront")
			assert.Contains(t, hdrIndex.Get("Content-Type"), "text/html", "GET /about/index.html should return HTML")

			// Same content for both paths
			assert.Equal(t, bodyIndex, bodySlash,
				"Requests to /about/ should be rewritten/handled to return the same contents as /about/index.html")
		})
	})

	// ---------------------------
	// Stage: destroy
	// ---------------------------
	defer test_structure.RunTestStage(t, "destroy", func() {
		log.Printf("[TF] Destroy starting…")
		terraform.Destroy(t, tfOpts)
		log.Printf("[TF] Destroy completed.")
	})
}
