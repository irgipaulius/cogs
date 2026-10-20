package blobstore

import (
	"bytes"
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3 is a generic S3-compatible store (MinIO on your host, etc.).
type S3 struct {
	client *s3.Client
	bucket string
}

// S3Config for MinIO or any S3 API endpoint.
type S3Config struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool
}

// NewS3 builds a client (MinIO: http://minio:9000, path-style).
func NewS3(ctx context.Context, cfg S3Config) (*S3, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return nil, fmt.Errorf("s3: incomplete config (endpoint, bucket, keys required)")
	}
	region := cfg.Region
	if region == "" {
		region = "auto"
	}
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, err
	}
	pathStyle := cfg.UsePathStyle
	if !pathStyle {
		pathStyle = true // MinIO and most self-hosted S3 need path-style
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = pathStyle
	})
	return &S3{client: client, bucket: cfg.Bucket}, nil
}

func (s *S3) Put(ctx context.Context, key string, data []byte, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	return err
}

func (s *S3) Get(ctx context.Context, key string) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(out.Body)
	return buf.Bytes(), err
}
