package user

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"testing"
)

type fakeAvatarBucket struct {
	name     string
	metadata map[string]string
	err      error
}

func (bucket *fakeAvatarBucket) Put(_ context.Context, name string, source io.Reader, _ string, metadata map[string]string) error {
	bucket.name, bucket.metadata = name, metadata
	_, _ = io.ReadAll(source)
	return bucket.err
}
func (bucket *fakeAvatarBucket) Delete(context.Context, string) error { return bucket.err }

func avatarPNG(t *testing.T) []byte {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, 1, 1))
	picture.Set(0, 0, color.RGBA{B: 1, A: 255})
	var output bytes.Buffer
	if err := png.Encode(&output, picture); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestFirebaseAvatarStoreUsesOutletUserPathAndDownloadToken(t *testing.T) {
	bucket := &fakeAvatarBucket{}
	store := NewFirebaseAvatarStore("dikasa.firebasestorage.app", bucket)
	content := avatarPNG(t)
	stored, err := store.Save(context.Background(), "outlet-a", "user-a", &AvatarUpload{Reader: bytes.NewReader(content), Size: int64(len(content)), ContentType: "image/png", Filename: "avatar.png"})
	if err != nil {
		t.Fatalf("save avatar: %v", err)
	}
	if !strings.HasPrefix(bucket.name, "outlets/outlet-a/users/user-a/avatar/") || bucket.metadata["firebaseStorageDownloadTokens"] == "" || !strings.Contains(stored.URL, "token=") {
		t.Fatalf("unexpected avatar object=%q metadata=%v stored=%+v", bucket.name, bucket.metadata, stored)
	}
}

func TestFirebaseAvatarStoreRejectsInvalidAndOversizedFiles(t *testing.T) {
	store := NewFirebaseAvatarStore("bucket", &fakeAvatarBucket{})
	if _, err := store.Save(context.Background(), "o", "u", &AvatarUpload{Reader: bytes.NewReader([]byte("bad")), Size: 3, ContentType: "image/png", Filename: "bad.png"}); !errors.Is(err, ErrInvalidAvatar) {
		t.Fatalf("invalid avatar: %v", err)
	}
	if _, err := store.Save(context.Background(), "o", "u", &AvatarUpload{Reader: bytes.NewReader(nil), Size: maxAvatarBytes + 1, ContentType: "image/png", Filename: "large.png"}); !errors.Is(err, ErrAvatarTooLarge) {
		t.Fatalf("large avatar: %v", err)
	}
}
