package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Config configures an S3-compatible object store.
type S3Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseTLS    bool
}

type s3Client interface {
	BucketExists(context.Context, string) (bool, error)
	MakeBucket(context.Context, string, minio.MakeBucketOptions) error
	PutObject(context.Context, string, string, io.Reader, int64, minio.PutObjectOptions) (minio.UploadInfo, error)
	GetObject(context.Context, string, string, minio.GetObjectOptions) (*minio.Object, error)
	RemoveObject(context.Context, string, string, minio.RemoveObjectOptions) error
}

// S3Store persists objects in an S3-compatible bucket.
type S3Store struct {
	client s3Client
	bucket string
}

// NewS3Store connects to S3-compatible storage and creates the bucket when absent.
func NewS3Store(ctx context.Context, cfg S3Config) (*S3Store, error) {
	if cfg.Endpoint == "" || cfg.AccessKey == "" || cfg.SecretKey == "" || cfg.Bucket == "" {
		return nil, errors.New("storage: S3 endpoint, credentials, and bucket are required")
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseTLS,
	})
	if err != nil {
		return nil, fmt.Errorf("storage: create S3 client: %w", err)
	}
	return newS3Store(ctx, client, cfg.Bucket)
}

func newS3Store(ctx context.Context, client s3Client, bucket string) (*S3Store, error) {
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("storage: check bucket %q: %w", bucket, err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			exists, checkErr := client.BucketExists(ctx, bucket)
			if checkErr != nil || !exists {
				return nil, fmt.Errorf("storage: create bucket %q: %w", bucket, err)
			}
		}
	}
	return &S3Store{client: client, bucket: bucket}, nil
}

func (s *S3Store) Put(ctx context.Context, key string, data []byte) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("storage: put S3 object %q: %w", key, err)
	}
	return nil
}

func (s *S3Store) Get(ctx context.Context, key string) ([]byte, error) {
	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, mapS3Error(key, err)
	}
	defer object.Close()
	data, err := io.ReadAll(object)
	if err != nil {
		return nil, mapS3Error(key, err)
	}
	return data, nil
}

func (s *S3Store) Delete(ctx context.Context, key string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("storage: delete S3 object %q: %w", key, err)
	}
	return nil
}

func mapS3Error(key string, err error) error {
	switch minio.ToErrorResponse(err).Code {
	case "NoSuchKey", "NoSuchObject", "NotFound":
		return fmt.Errorf("%w: %s", ErrNotFound, key)
	default:
		return fmt.Errorf("storage: get S3 object %q: %w", key, err)
	}
}
