package blobstore

import (
	"context"

	"github.com/irgipaulius/cogs/pkg/r2"
)

// R2 adapts pkg/r2 for blob storage.
type R2 struct {
	Client *r2.Client
}

func (r R2) Put(ctx context.Context, key string, data []byte, contentType string) error {
	return r.Client.PutBytes(ctx, key, data, contentType)
}

func (r R2) Get(ctx context.Context, key string) ([]byte, error) {
	return r.Client.GetBytes(ctx, key)
}
