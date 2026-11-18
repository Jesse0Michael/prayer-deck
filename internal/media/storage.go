// Package media stores card artwork in S3-compatible object storage and
// prepares the renditions each upload is served as.
package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/jesse0michael/pkg/config"
)

// Config controls artwork URL lifetimes and local bucket provisioning.
type Config struct {
	URLExpiry time.Duration `envconfig:"STORAGE_URL_EXPIRY" default:"6h"`
	// CreateBucket creates the bucket at startup when it is missing; meant for local MinIO.
	CreateBucket bool `envconfig:"STORAGE_CREATE_BUCKET"`
}

// Storage reads and writes artwork objects in one bucket.
type Storage struct {
	client *s3.Client
	// signer only signs URLs. Signing happens offline, so this client never
	// needs to reach the public endpoint from inside the deployment.
	signer *s3.PresignClient
	bucket string
	expiry time.Duration
}

// NewStorage connects to the configured bucket, creating it when CreateBucket is set.
func NewStorage(ctx context.Context, awsCfg aws.Config, s3Cfg config.S3Config, cfg Config) (*Storage, error) {
	if s3Cfg.Bucket == "" {
		return nil, errors.New("storage bucket is required")
	}
	if cfg.URLExpiry <= 0 || cfg.URLExpiry > 7*24*time.Hour {
		return nil, errors.New("storage URL expiry must be greater than zero and at most 168h")
	}
	client := config.NewS3Client(awsCfg, s3Cfg)
	signer := config.NewS3PresignClient(awsCfg, s3Cfg)

	if cfg.CreateBucket {
		_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: new(s3Cfg.Bucket)})
		if err != nil {
			var missing *types.NotFound
			if !errors.As(err, &missing) {
				return nil, fmt.Errorf("failed to check bucket %s: %w", s3Cfg.Bucket, err)
			}
			input := &s3.CreateBucketInput{Bucket: new(s3Cfg.Bucket)}
			// S3 requires the location constraint to be omitted for us-east-1.
			if region := client.Options().Region; region != "" && region != "us-east-1" {
				input.CreateBucketConfiguration = &types.CreateBucketConfiguration{LocationConstraint: types.BucketLocationConstraint(region)}
			}
			if _, err := client.CreateBucket(ctx, input); err != nil {
				return nil, fmt.Errorf("failed to create bucket %s: %w", s3Cfg.Bucket, err)
			}
		}
	}

	return &Storage{client: client, signer: signer, bucket: s3Cfg.Bucket, expiry: cfg.URLExpiry}, nil
}

// Put uploads an object.
func (s *Storage) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        new(s.bucket),
		Key:           new(key),
		Body:          r,
		ContentLength: new(size),
		ContentType:   new(contentType),
		// Keys are never reused, so an object at a key never changes.
		CacheControl: new("private, max-age=31536000, immutable"),
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
		if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: new(s.bucket), Key: new(key)}); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete object %s: %w", key, err))
		}
	}
	return errors.Join(errs...)
}

// URL returns a time-limited URL a browser can load the object from.
func (s *Storage) URL(ctx context.Context, key string) (string, error) {
	u, err := s.signer.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: new(s.bucket), Key: new(key)}, s3.WithPresignExpires(s.expiry))
	if err != nil {
		return "", fmt.Errorf("failed to sign url for %s: %w", key, err)
	}
	return u.URL, nil
}

// Healthy reports whether the bucket is reachable.
func (s *Storage) Healthy(ctx context.Context) error {
	if _, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: new(s.bucket)}); err != nil {
		return fmt.Errorf("failed to reach bucket %s: %w", s.bucket, err)
	}
	return nil
}
