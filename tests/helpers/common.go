package helpers

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"

	"github.com/aws/aws-sdk-go-v2/aws"
)

var (
	onceAWS sync.Once
	onceTF  sync.Once

	awsCfg    aws.Config
	awsCtx    context.Context
	awsRegion string

	tfOpts *terraform.Options
	paths  Paths
)

type Paths struct {
	ThisDir string
	TfDir   string
	VarFile string
}

func getEnvOrDefault(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func DiscoverPaths(t *testing.T) Paths {
	t.Helper()
	if paths == (Paths{}) {
		_, thisFile, _, _ := runtime.Caller(0)
		thisDir := filepath.Dir(thisFile)
		paths = Paths{
			ThisDir: thisDir,
			TfDir:   filepath.Clean(filepath.Join(thisDir, "../..")),
			VarFile: filepath.Clean(filepath.Join(thisDir, "test.tfvars")),
		}
	}
	return paths
}

func TFOptions(t *testing.T) *terraform.Options {
	t.Helper()
	onceTF.Do(func() {
		p := DiscoverPaths(t)
		region := getEnvOrDefault("AWS_REGION", "eu-west-2")
		profile := getEnvOrDefault("AWS_PROFILE", "static-site-test")
		awsRegion = region

		tfOpts = terraform.WithDefaultRetryableErrors(t, &terraform.Options{
			TerraformDir: p.TfDir,
			EnvVars: map[string]string{
				"AWS_PROFILE": profile,
				"AWS_REGION":  region,
			},
			VarFiles: []string{p.VarFile},
		})
	})
	return tfOpts
}

// Terraform outputs
func TFOutput(t *testing.T, key string) string {
	t.Helper()
	return terraform.Output(t, TFOptions(t), key)
}
