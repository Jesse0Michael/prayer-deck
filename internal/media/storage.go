// Package media stores card artwork in S3-compatible object storage and
// prepares the renditions each upload is served as.
package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Config configures the S3-compatible artwork bucket.
type Config struct {
	// Endpoint is the URL the API reaches the object store at.
	Endpoint string `envconfig:"STORAGE_ENDPOINT" default:"http://localhost:9000"`
	// PublicEndpoint is the URL browsers reach the object store at, used to
	// sign artwork URLs. Defaults to Endpoint.
	PublicEndpoint string        `envconfig:"STORAGE_PUBLIC_ENDPOINT"`
	Region         string        `envconfig:"STORAGE_REGION" default:"us-east-1"`
	Bucket         string        `envconfig:"STORAGE_BUCKET" default:"prayer-deck"`
	AccessKey      string        `envconfig:"STORAGE_ACCESS_KEY"`
	SecretKey      string        `envconfig:"STORAGE_SECRET_KEY"`
	URLExpiry      time.Duration `envconfig:"STORAGE_URL_EXPIRY" default:"6h"`
	// CreateBucket creates the bucket at startup when it is missing; meant for local MinIO.
	CreateBucket bool `envconfig:"STORAGE_CREATE_BUCKET"`
}

// Storage reads and writes artwork objects in one bucket.
type Storage struct {
	client *minio.Client
	// signer only signs URLs. Signing happens offline, so this client never
	// needs to reach the public endpoint from inside the deployment.
	signer *minio.Client
	bucket string
	expiry time.Duration
}

// NewStorage connects to the configured bucket, creating it when CreateBucket is set.
func NewStorage(ctx context.Context, cfg Config) (*Storage, error) {
	client, err := newClient(cfg.Endpoint, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %w", err)
	}
	public := cfg.PublicEndpoint
	if public == "" {
		public = cfg.Endpoint
	}
	signer, err := newClient(public, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage signer: %w", err)
	}

	if cfg.CreateBucket {
		exists, err := client.BucketExists(ctx, cfg.Bucket)
		if err != nil {
			return nil, fmt.Errorf("failed to check bucket %s: %w", cfg.Bucket, err)
		}
		if !exists {
			if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{Region: cfg.Region}); err != nil {
				return nil, fmt.Errorf("failed to create bucket %s: %w", cfg.Bucket, err)
			}
		}
	}

	return &Storage{client: client, signer: signer, bucket: cfg.Bucket, expiry: cfg.URLExpiry}, nil
}

func newClient(endpoint string, cfg Config) (*minio.Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse endpoint %q: %w", endpoint, err)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("endpoint %q must be a URL with a scheme and host", endpoint)
	}
	return minio.New(u.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: u.Scheme == "https",
		// A fixed region keeps URL signing from looking the region up over the network.
		Region: cfg.Region,
	})
}

// Put uploads an object.
func (s *Storage) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{
		ContentType: contentType,
		// Keys are never reused, so an object at a key never changes.
		CacheControl: "private, max-age=31536000, immutable",
	})
	if err != nil {
		return fmt.Errorf("failed to put object %s: %w", key, err)
	}
	return nil
}

// Delete removes objects, attempting every key and reporting all failures.
func (s *Storage) Delete(ctx context.Context, keys ...string) error {
	var errs []error
	for _, key := range keys {
		if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete object %s: %w", key, err))
		}
	}
	return errors.Join(errs...)
}

// URL returns a time-limited URL a browser can load the object from.
func (s *Storage) URL(ctx context.Context, key string) (string, error) {
	u, err := s.signer.PresignedGetObject(ctx, s.bucket, key, s.expiry, nil)
	if err != nil {
		return "", fmt.Errorf("failed to sign url for %s: %w", key, err)
	}
	return u.String(), nil
}

// Healthy reports whether the bucket is reachable.
func (s *Storage) Healthy(ctx context.Context) error {
	if _, err := s.client.BucketExists(ctx, s.bucket); err != nil {
		return fmt.Errorf("failed to reach bucket %s: %w", s.bucket, err)
	}
	return nil
}
