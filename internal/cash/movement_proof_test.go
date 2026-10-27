package cash

import (
	"be-dikasa/internal/shared/firebaseimage"
	"be-dikasa/internal/shared/imagefile"
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"strings"
	"testing"
)

type fakeProofBucket struct {
	name, contentType string
	metadata          map[string]string
	content           []byte
	deleted           string
}

func (bucket *fakeProofBucket) Put(_ context.Context, name string, source io.Reader, contentType string, metadata map[string]string) error {
	bucket.name, bucket.contentType, bucket.metadata = name, contentType, metadata
	bucket.content, _ = io.ReadAll(source)
	return nil
}

func (bucket *fakeProofBucket) Delete(_ context.Context, name string) error {
	bucket.deleted = name
	return nil
}

func TestFirebaseMovementProofStoreUsesSharedDownloadURL(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	validated, err := imagefile.Validate(imagefile.Upload{Reader: bytes.NewReader(encoded.Bytes()), Size: int64(encoded.Len()), ContentType: "image/png", Filename: "proof.png"}, imagefile.StandardPolicy)
	if err != nil {
		t.Fatal(err)
	}
	bucket := &fakeProofBucket{}
	store := NewFirebaseMovementProofStore(firebaseimage.NewUploader("dikasa.firebasestorage.app", bucket))
	object, err := store.Save(context.Background(), "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", validated)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(bucket.name, "outlets/11111111-1111-4111-8111-111111111111/cash-sessions/22222222-2222-4222-8222-222222222222/proofs/") {
		t.Fatalf("unexpected proof object %q", bucket.name)
	}
	if bucket.metadata["firebaseStorageDownloadTokens"] == "" || !strings.HasPrefix(object.URL, "https://firebasestorage.googleapis.com/v0/b/") {
		t.Fatalf("proof did not use shared Firebase URL mechanism: metadata=%v url=%q", bucket.metadata, object.URL)
	}
	if err = store.Delete(context.Background(), object.URL); err != nil || bucket.deleted != bucket.name {
		t.Fatalf("delete failed: key=%q error=%v", bucket.deleted, err)
	}
}
