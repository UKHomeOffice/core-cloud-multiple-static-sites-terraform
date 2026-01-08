package helpers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

var (
	awsOnce sync.Once
	tfOnce  sync.Once

	awsConfig  aws.Config
	awsContext context.Context
	awsRegion  string

	tfOpts    *terraform.Options
	paths     Paths
	pathsOnce sync.Once
)

type Paths struct {
	ThisDir string
	TfDir   string
	VarFile string
}

// AWSConfig returns the AWS configuration, context, and region.
func AWSConfig(t *testing.T) (context.Context, aws.Config, string) {
	t.Helper()
	awsOnce.Do(func() {
		awsContext = context.Background()
		cfg, err := config.LoadDefaultConfig(awsContext, config.WithRegion(awsRegion))
		require.NoError(t, err)
		awsConfig = cfg
	})
	return awsContext, awsConfig, awsRegion
}

// GetEnvironmentValueOrSetDefault retrieves the value of an environment variable or returns a default value.
func GetEnvironmentValueOrSetDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// GetPaths retrieves the file paths used in the tests.
func GetPaths() Paths {
	pathsOnce.Do(func() {
		_, thisFile, _, _ := runtime.Caller(0)
		thisDir := filepath.Dir(thisFile)

		paths = Paths{
			ThisDir: thisDir,
			TfDir:   filepath.Join(thisDir, "../.."),
			VarFile: filepath.Join(thisDir, "test.tfvars"),
		}
	})

	return paths
}

// TFOptions returns the Terraform options for the tests.
func TFOptions(t *testing.T) *terraform.Options {
	t.Helper()
	tfOnce.Do(func() {
		path := GetPaths()
		region := GetEnvironmentValueOrSetDefault("AWS_REGION", "eu-west-2")
		profile := GetEnvironmentValueOrSetDefault("AWS_PROFILE", "static-site-test")
		awsRegion = region

		tfOpts = terraform.WithDefaultRetryableErrors(t, &terraform.Options{
			TerraformDir: path.TfDir,
			EnvVars: map[string]string{
				"AWS_PROFILE": profile,
				"AWS_REGION":  region,
			},
			VarFiles: []string{path.VarFile},
		})
	})
	return tfOpts
}

// TFOutput returns the value of a Terraform output variable.
func TFOutput(t *testing.T, key string) string {
	t.Helper()
	return terraform.Output(t, TFOptions(t), key)
}

// HttpGetWithRetry performs a GET request with retries.
func HttpGetWithRetry(
	t *testing.T,
	url string,
	attempts int,
	delay time.Duration,
	expectedStatus int,
) (int, string, http.Header) {

	t.Helper()

	client := &http.Client{Timeout: 15 * time.Second}
	var lastErr error

	for i := 0; i < attempts; i++ {
		response, err := client.Get(url)
		if err != nil {
			lastErr = err
			time.Sleep(delay)
			continue
		}

		bodyBytes, _ := io.ReadAll(response.Body)
		response.Body.Close()
		body := string(bodyBytes)

		// Success: expected status is matched
		if response.StatusCode == expectedStatus {
			return response.StatusCode, body, response.Header
		}

		lastErr = fmt.Errorf("expected %d, got %d", expectedStatus, response.StatusCode)
		time.Sleep(delay)
	}

	require.Failf(t, "http retries exhausted",
		"GET %s failed after %d attempts: last error: %v",
		url, attempts, lastErr,
	)
	return 0, "", nil
}
