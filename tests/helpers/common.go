package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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

		useVarFile := fileExists(path.VarFile)
		if useVarFile {
			log.Printf("[HELPERS] Using var-file: %s", path.VarFile)
		} else {
			log.Printf("[HELPERS] var-file not found, falling back to TF_VAR_* envs")
		}

		// Build Vars from secrets
		vars := map[string]interface{}{
			"aws_region": getEnvOrDefault("TF_VAR_AWS_REGION", "eu-west-2"),
			"env_name":   getEnvOrDefault("TF_VAR_ENV_NAME", "automation-test"),
			"cloudfront_function_name": getEnvOrDefault("TF_VAR_CLOUDFRONT_FUNCTION_NAME",
				"StaticSiteReWriteDefaultIndexRequest-automation-test"),
			"cloud_front_default_vars": decodeJSONOrEmptyMap("TF_VAR_CLOUD_FRONT_DEFAULT_VARS"),
			"platform_tags":            decodeJSONOrEmptyMap("TF_VAR_PLATFORM_TAGS"),
			"tenant_vars":              decodeJSONOrEmptyMap("TF_VAR_TENANT_VARS"),
		}

		opts := &terraform.Options{
			TerraformDir: path.TfDir,
			EnvVars: map[string]string{
				// Needed for local run
				// "AWS_PROFILE":      "static-site-test",
				// Set this to false for local run for additional information
				"TF_IN_AUTOMATION": getEnvOrDefault("TF_IN_AUTOMATION", "true"),
			},
			VarFiles: func() []string {
				if useVarFile {
					return []string{path.VarFile}
				}
				return nil
			}(),
			Vars: vars,
		}

		tfOpts = terraform.WithDefaultRetryableErrors(t, opts)
	})

	return tfOpts
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func getEnvOrDefault(key, value string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return value
}

// Reads JSON object from env var `key` into map[string]interface{}.
// If empty/invalid, returns empty map and logs a warning.
func decodeJSONOrEmptyMap(key string) map[string]interface{} {
	value := os.Getenv(key)
	if value == "" {
		return map[string]interface{}{}
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(value), &out); err != nil {
		log.Printf("[HELPERS] WARNING: invalid JSON in %s: %v; using empty object", key, err)
		return map[string]interface{}{}
	}
	return out
}
