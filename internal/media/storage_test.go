package media

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/jesse0michael/pkg/config"
	"github.com/jesse0michael/pkg/test"
	"github.com/stretchr/testify/require"
)

func storageAWSConfig() aws.Config {
	return aws.Config{
		Region: "us-east-1",
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: "test-access-key", SecretAccessKey: "test-secret-key"}, nil
		}),
		RetryMaxAttempts: 1,
	}
}

func TestNewStorage(t *testing.T) {
	type request struct {
		Method   string
		Path     string
		Location string
	}
	tests := []struct {
		name        string
		bucket      string
		expiry      time.Duration
		create      bool
		region      string
		headStatus  int
		putStatus   int
		expected    []request
		errorPrefix string
	}{
		{name: "provisioning disabled", bucket: "test-bucket", expiry: time.Hour},
		{name: "missing bucket", expiry: time.Hour, errorPrefix: "storage bucket is required"},
		{name: "zero expiry", bucket: "test-bucket", errorPrefix: "storage URL expiry"},
		{name: "negative expiry", bucket: "test-bucket", expiry: -time.Hour, errorPrefix: "storage URL expiry"},
		{name: "expiry over seven days", bucket: "test-bucket", expiry: 169 * time.Hour, errorPrefix: "storage URL expiry"},
		{
			name: "existing bucket", bucket: "test-bucket", expiry: time.Hour, create: true,
			headStatus: http.StatusOK, expected: []request{{Method: "HEAD", Path: "/test-bucket"}},
		},
		{
			name: "missing bucket in default region", bucket: "test-bucket", expiry: time.Hour, create: true,
			headStatus: http.StatusNotFound, putStatus: http.StatusOK,
			expected: []request{{Method: "HEAD", Path: "/test-bucket"}, {Method: "PUT", Path: "/test-bucket"}},
		},
		{
			name: "missing bucket in configured region", bucket: "test-bucket", expiry: time.Hour, create: true, region: "eu-west-1",
			headStatus: http.StatusNotFound, putStatus: http.StatusOK,
			expected: []request{{Method: "HEAD", Path: "/test-bucket"}, {Method: "PUT", Path: "/test-bucket", Location: "eu-west-1"}},
		},
		{
			name: "access denied does not create bucket", bucket: "test-bucket", expiry: time.Hour, create: true,
			headStatus: http.StatusForbidden, expected: []request{{Method: "HEAD", Path: "/test-bucket"}},
			errorPrefix: "failed to check bucket test-bucket",
		},
		{
			name: "bucket creation fails", bucket: "test-bucket", expiry: time.Hour, create: true,
			headStatus: http.StatusNotFound, putStatus: http.StatusForbidden,
			expected:    []request{{Method: "HEAD", Path: "/test-bucket"}, {Method: "PUT", Path: "/test-bucket"}},
			errorPrefix: "failed to create bucket test-bucket",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mu sync.Mutex
			var requests []request
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Location string `xml:"LocationConstraint"`
				}
				if r.Method == http.MethodPut && r.ContentLength > 0 {
					if err := xml.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Errorf("decode bucket location: %v", err)
					}
				}
				mu.Lock()
				requests = append(requests, request{Method: r.Method, Path: r.URL.Path, Location: body.Location})
				mu.Unlock()
				if r.Method == http.MethodHead {
					w.WriteHeader(tt.headStatus)
					return
				}
				w.WriteHeader(tt.putStatus)
			}))
			t.Cleanup(srv.Close)

			storage, err := NewStorage(t.Context(), storageAWSConfig(), config.S3Config{Bucket: tt.bucket, Endpoint: srv.URL, Region: tt.region}, Config{URLExpiry: tt.expiry, CreateBucket: tt.create})

			if tt.errorPrefix != "" {
				require.ErrorContains(t, err, tt.errorPrefix)
				require.Nil(t, storage)
			} else {
				require.NoError(t, err)
				require.NotNil(t, storage)
			}
			mu.Lock()
			defer mu.Unlock()
			require.Equal(t, tt.expected, requests)
		})
	}
}

func TestStorage_URL(t *testing.T) {
	tests := []struct {
		name           string
		endpoint       string
		publicEndpoint string
		region         string
		expiry         time.Duration
		expectedOrigin string
		expectedExpiry string
	}{
		{name: "public endpoint", endpoint: "http://minio.invalid:9000", publicEndpoint: "https://artwork.invalid", region: "us-east-1", expiry: 6 * time.Hour, expectedOrigin: "https://artwork.invalid", expectedExpiry: "21600"},
		{name: "internal endpoint fallback", endpoint: "http://minio.invalid:9000", region: "us-east-1", expiry: time.Hour, expectedOrigin: "http://minio.invalid:9000", expectedExpiry: "3600"},
		{name: "R2 region and maximum expiry", endpoint: "https://test-account.r2.cloudflarestorage.com", region: "auto", expiry: 168 * time.Hour, expectedOrigin: "https://test-account.r2.cloudflarestorage.com", expectedExpiry: "604800"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, err := NewStorage(t.Context(), storageAWSConfig(), config.S3Config{Bucket: "test-bucket", Endpoint: tt.endpoint, PublicEndpoint: tt.publicEndpoint, Region: tt.region}, Config{URLExpiry: tt.expiry})
			require.NoError(t, err)

			signed, err := storage.URL(t.Context(), "test-card/test image.png")
			require.NoError(t, err)
			u, err := url.Parse(signed)
			require.NoError(t, err)
			require.Equal(t, tt.expectedOrigin, u.Scheme+"://"+u.Host)
			require.Equal(t, "/test-bucket/test-card/test%20image.png", u.EscapedPath())
			require.Equal(t, tt.expectedExpiry, u.Query().Get("X-Amz-Expires"))
			credential := strings.Split(u.Query().Get("X-Amz-Credential"), "/")
			require.Len(t, credential, 5)
			require.Equal(t, "test-access-key", credential[0])
			require.Equal(t, []string{tt.region, "s3", "aws4_request"}, credential[2:])
			require.NotEmpty(t, u.Query().Get("X-Amz-Signature"))
		})
	}
}

func TestStorage_Put(t *testing.T) {
	type upload struct {
		Method        string
		Path          string
		ContentLength int64
		ContentType   string
		CacheControl  string
		Body          []byte
	}
	tests := []struct {
		name   string
		status int
	}{
		{name: "uploaded", status: http.StatusOK},
		{name: "access denied", status: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := test.LoadFile(t, "testdata/small.png")
			var mu sync.Mutex
			var uploads []upload
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("read artwork: %v", err)
				}
				mu.Lock()
				uploads = append(uploads, upload{
					Method: r.Method, Path: r.URL.Path, ContentLength: r.ContentLength,
					ContentType: r.Header.Get("Content-Type"), CacheControl: r.Header.Get("Cache-Control"), Body: body,
				})
				mu.Unlock()
				w.WriteHeader(tt.status)
			}))
			t.Cleanup(srv.Close)
			storage, err := NewStorage(t.Context(), storageAWSConfig(), config.S3Config{Bucket: "test-bucket", Endpoint: srv.URL}, Config{URLExpiry: time.Hour})
			require.NoError(t, err)

			err = storage.Put(t.Context(), "test-card/test-image.png", bytes.NewReader(data), int64(len(data)), "image/png")

			if tt.status == http.StatusOK {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "failed to put object test-card/test-image.png")
			}
			mu.Lock()
			defer mu.Unlock()
			require.Equal(t, []upload{{
				Method: http.MethodPut, Path: "/test-bucket/test-card/test-image.png", ContentLength: int64(len(data)),
				ContentType: "image/png", CacheControl: "private, max-age=31536000, immutable", Body: data,
			}}, uploads)
		})
	}
}

func TestStorage_Delete(t *testing.T) {
	tests := []struct {
		name   string
		keys   []string
		failed []string
	}{
		{name: "no keys"},
		{name: "all deleted", keys: []string{"test-image-1", "test-image-2"}},
		{name: "attempts every key and joins errors", keys: []string{"test-image-1", "test-image-2", "test-image-3"}, failed: []string{"test-image-1", "test-image-3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mu sync.Mutex
			var deleted []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					t.Errorf("unexpected method: %s", r.Method)
				}
				key := strings.TrimPrefix(r.URL.Path, "/test-bucket/")
				mu.Lock()
				deleted = append(deleted, key)
				mu.Unlock()
				for _, failed := range tt.failed {
					if key == failed {
						w.WriteHeader(http.StatusForbidden)
						return
					}
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			t.Cleanup(srv.Close)
			storage, err := NewStorage(t.Context(), storageAWSConfig(), config.S3Config{Bucket: "test-bucket", Endpoint: srv.URL}, Config{URLExpiry: time.Hour})
			require.NoError(t, err)

			err = storage.Delete(t.Context(), tt.keys...)

			if len(tt.failed) == 0 {
				require.NoError(t, err)
			} else {
				for _, key := range tt.failed {
					require.ErrorContains(t, err, "failed to delete object "+key)
				}
			}
			mu.Lock()
			defer mu.Unlock()
			require.Equal(t, tt.keys, deleted)
		})
	}
}

func TestStorage_Healthy(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{name: "bucket available", status: http.StatusOK},
		{name: "missing bucket", status: http.StatusNotFound},
		{name: "access denied", status: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodHead || r.URL.Path != "/test-bucket" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(tt.status)
			}))
			t.Cleanup(srv.Close)
			storage, err := NewStorage(t.Context(), storageAWSConfig(), config.S3Config{Bucket: "test-bucket", Endpoint: srv.URL}, Config{URLExpiry: time.Hour})
			require.NoError(t, err)

			err = storage.Healthy(t.Context())

			if tt.status == http.StatusOK {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "failed to reach bucket test-bucket")
			}
		})
	}
}
