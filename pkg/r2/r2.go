// Package r2 wraps Cloudflare R2 via the S3 API with optional zstd compression.
package r2

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/klauspost/compress/zstd"
)

// Client talks to an R2 bucket.
type Client struct {
	S3     *s3.Client
	Bucket string
}

// Config holds R2 credentials.
type Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
}

// New builds an R2 S3 client.
func New(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.AccountID == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" || cfg.Bucket == "" {
		return nil, fmt.Errorf("r2: incomplete config")
	}
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, err
	}
	s3c := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})
	return &Client{S3: s3c, Bucket: cfg.Bucket}, nil
}

// PutCompressed stores zstd-compressed data at key, returning content hash.
func (c *Client) PutCompressed(ctx context.Context, key string, plain []byte) (sha256Hex string, err error) {
	var buf bytes.Buffer
	enc, err := zstd.NewWriter(&buf)
	if err != nil {
		return "", err
	}
	if _, err := enc.Write(plain); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	body := buf.Bytes()
	sum := sha256.Sum256(body)
	sha256Hex = hex.EncodeToString(sum[:])
	_, err = c.S3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:          aws.String(c.Bucket),
		Key:             aws.String(key),
		Body:            bytes.NewReader(body),
		ContentType:     aws.String("application/octet-stream"),
		ContentEncoding: aws.String("zstd"),
		Metadata: map[string]string{
			"cogs-sha256": sha256Hex,
		},
	})
	return sha256Hex, err
}

// HeadSHA256 returns the cogs-sha256 metadata if the object exists.
func (c *Client) HeadSHA256(ctx context.Context, key string) (string, bool, error) {
	out, err := c.S3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return "", false, err
	}
	if out.Metadata != nil {
		if h, ok := out.Metadata["cogs-sha256"]; ok {
			return h, true, nil
		}
	}
	return "", true, nil
}

// GetBytes reads a raw object body.
func (c *Client) GetBytes(ctx context.Context, key string) ([]byte, error) {
	out, err := c.S3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

// GetDecompressed reads and decompresses a zstd object.
func (c *Client) GetDecompressed(ctx context.Context, key string) ([]byte, error) {
	out, err := c.S3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	comp, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, err
	}
	dec, err := zstd.NewReader(bytes.NewReader(comp))
	if err != nil {
		return nil, err
	}
	defer dec.Close()
	return io.ReadAll(dec)
}

// PutBytes stores raw bytes at key with optional content type.
func (c *Client) PutBytes(ctx context.Context, key string, body []byte, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	_, err := c.S3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(contentType),
	})
	return err
}

// PutIfAbsent uploads only when key is missing (If-None-Match: *).
func (c *Client) PutIfAbsent(ctx context.Context, key string, body []byte, contentType string) error {
	_, err := c.S3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(contentType),
		IfNoneMatch: aws.String("*"),
	})
	return err
}
