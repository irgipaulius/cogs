package blobstore

import (
	"context"
	"fmt"

	"github.com/irgipaulius/cogs/pkg/config"
	"github.com/irgipaulius/cogs/pkg/r2"
)

// Store reads and writes attachment blobs.
type Store interface {
	Put(ctx context.Context, key string, data []byte, contentType string) error
	Get(ctx context.Context, key string) ([]byte, error)
}

// New selects local filesystem (default), MinIO/S3, or R2 when fully configured.
func New(ctx context.Context, cfg config.App, r2c *r2.Client) (Store, error) {
	switch cfg.BlobStore {
	case "r2":
		if r2c == nil {
			return nil, fmt.Errorf("COGS_BLOB_STORE=r2 but R2 credentials are incomplete")
		}
		return R2{Client: r2c}, nil
	case "s3", "minio":
		s, err := NewS3(ctx, S3Config{
			Endpoint:        cfg.S3Endpoint,
			Region:          cfg.S3Region,
			Bucket:          cfg.S3Bucket,
			AccessKeyID:     cfg.S3AccessKeyID,
			SecretAccessKey: cfg.S3SecretAccessKey,
			UsePathStyle:    cfg.S3PathStyle,
		})
		if err != nil {
			return nil, err
		}
		return s, nil
	default:
		dir := cfg.BlobDir
		if dir == "" {
			dir = "data/blobs"
		}
		return FS{Root: dir}, nil
	}
}
