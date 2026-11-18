package main

import (
	"os"
	"testing"

	"github.com/jesse0michael/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestConfigStorageAliases(t *testing.T) {
	legacy := map[string]string{
		"STORAGE_ACCESS_KEY":      "test-legacy-access",
		"STORAGE_SECRET_KEY":      "test-legacy-secret",
		"STORAGE_ENDPOINT":        "https://test-legacy.invalid",
		"STORAGE_PUBLIC_ENDPOINT": "https://test-legacy-public.invalid",
		"STORAGE_REGION":          "auto",
		"STORAGE_BUCKET":          "test-legacy-bucket",
	}
	current := map[string]string{
		"AWS_ACCESS_KEY_ID":     "test-current-access",
		"AWS_SECRET_ACCESS_KEY": "test-current-secret",
		"AWS_ENDPOINT_URL_S3":   "https://test-current.invalid",
		"S3_PUBLIC_ENDPOINT":    "https://test-current-public.invalid",
		"S3_REGION":             "us-east-1",
		"S3_BUCKET":             "test-current-bucket",
	}
	tests := []struct {
		name    string
		legacy  map[string]string
		current map[string]string
		aws     config.AWSConfig
		s3      config.S3Config
	}{
		{
			name:   "legacy configuration",
			legacy: legacy,
			aws:    config.AWSConfig{AccessKey: "test-legacy-access", SecretKey: "test-legacy-secret"},
			s3:     config.S3Config{Endpoint: "https://test-legacy.invalid", PublicEndpoint: "https://test-legacy-public.invalid", Region: "auto", Bucket: "test-legacy-bucket"},
		},
		{
			name:    "current configuration",
			current: current,
			aws:     config.AWSConfig{AccessKey: "test-current-access", SecretKey: "test-current-secret"},
			s3:      config.S3Config{Endpoint: "https://test-current.invalid", PublicEndpoint: "https://test-current-public.invalid", Region: "us-east-1", Bucket: "test-current-bucket"},
		},
		{
			name:    "current configuration takes precedence",
			legacy:  legacy,
			current: current,
			aws:     config.AWSConfig{AccessKey: "test-current-access", SecretKey: "test-current-secret"},
			s3:      config.S3Config{Endpoint: "https://test-current.invalid", PublicEndpoint: "https://test-current-public.invalid", Region: "us-east-1", Bucket: "test-current-bucket"},
		},
		{
			name:    "explicit empty endpoint takes precedence",
			legacy:  map[string]string{"STORAGE_ENDPOINT": "https://test-legacy.invalid"},
			current: map[string]string{"AWS_ENDPOINT_URL_S3": ""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AUTH_SECRET_KEY", "test-auth-secret")
			for _, key := range []string{
				"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_REGION", "AWS_PROFILE", "AWS_ENDPOINT_URL_S3",
				"S3_PUBLIC_ENDPOINT", "S3_REGION", "S3_BUCKET",
				"STORAGE_ACCESS_KEY", "STORAGE_SECRET_KEY", "STORAGE_ENDPOINT", "STORAGE_PUBLIC_ENDPOINT", "STORAGE_REGION", "STORAGE_BUCKET",
			} {
				t.Setenv(key, "")
				require.NoError(t, os.Unsetenv(key))
			}
			for key, value := range tt.legacy {
				t.Setenv(key, value)
			}
			for key, value := range tt.current {
				t.Setenv(key, value)
			}

			cfg, err := config.New[Config]()

			require.NoError(t, err)
			require.Equal(t, tt.aws, cfg.AWS)
			require.Equal(t, tt.s3, cfg.S3)
		})
	}
}
