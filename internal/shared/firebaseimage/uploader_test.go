package firebaseimage

import (
	"bytes"
	"context"
	"io"
	"testing"
)

type fakeBucket struct {
	name, deleted string
	content       []byte
	metadata      map[string]string
}

func (bucket *fakeBucket) Put(_ context.Context, name string, source io.Reader, _ string, metadata map[string]string) error {
	bucket.name, bucket.metadata = name, metadata
	bucket.content, _ = io.ReadAll(source)
	return nil
}

func (bucket *fakeBucket) Delete(_ context.Context, name string) error {
	bucket.deleted = name
	return nil
}

func TestUploaderReturnsFirebaseURLAndDeletesByURL(t *testing.T) {
	bucket := &fakeBucket{}
	uploader := NewUploader("dikasa-dev.firebasestorage.app", bucket)
	objectName := "outlets/11111111-1111-4111-8111-111111111111/products/22222222-2222-4222-8222-222222222222/images/33333333-3333-4333-8333-333333333333.png"
	url, err := uploader.Upload(context.Background(), objectName, bytes.NewReader([]byte("png")), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if bucket.name != objectName || bucket.metadata["firebaseStorageDownloadTokens"] == "" {
		t.Fatalf("unexpected upload: name=%q metadata=%v", bucket.name, bucket.metadata)
	}
	if err = uploader.DeleteURL(context.Background(), url); err != nil || bucket.deleted != objectName {
		t.Fatalf("delete by URL failed: deleted=%q error=%v", bucket.deleted, err)
	}
}

func TestUploaderRejectsNonFirebaseOrForeignBucketURL(t *testing.T) {
	uploader := NewUploader("dikasa-dev.firebasestorage.app", &fakeBucket{})
	for _, value := range []string{
		"https://example.com/image.png",
		"https://firebasestorage.googleapis.com/v0/b/other.firebasestorage.app/o/image.png?alt=media&token=x",
		"https://firebasestorage.googleapis.com/v0/b/dikasa-dev.firebasestorage.app/o/image.png?alt=media",
	} {
		if _, err := uploader.ObjectName(value); err == nil {
			t.Fatalf("accepted untrusted URL %q", value)
		}
	}
}
