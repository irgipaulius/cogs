package api

import (
	"context"

	"github.com/irgipaulius/cogs/pkg/blobstore"
)

type blobCache struct {
	s blobstore.Store
}

func BlobCacheFrom(s blobstore.Store) BlobCache {
	if s == nil {
		return noopBlob{}
	}
	return blobCache{s: s}
}

func (b blobCache) Get(key string) ([]byte, bool) {
	data, err := b.s.Get(context.Background(), key)
	if err != nil {
		return nil, false
	}
	return data, true
}

func (b blobCache) Put(key string, data []byte) error {
	return b.s.Put(context.Background(), key, data, "application/json")
}
