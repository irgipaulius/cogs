package blobstore

import (
	"context"
	"testing"
)

func TestFSRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := FS{Root: dir}
	ctx := context.Background()
	if err := s.Put(ctx, "todo-images/a/photo.png", []byte("png"), "image/png"); err != nil {
		t.Fatal(err)
	}
	b, err := s.Get(ctx, "todo-images/a/photo.png")
	if err != nil || string(b) != "png" {
		t.Fatalf("get %q err=%v", b, err)
	}
}

func TestFSRejectTraversal(t *testing.T) {
	s := FS{Root: t.TempDir()}
	if err := s.Put(context.Background(), "../escape", []byte("x"), ""); err == nil {
		t.Fatal("expected reject")
	}
}
