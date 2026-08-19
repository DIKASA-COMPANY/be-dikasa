package catalog

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

type fakeObjectBucket struct {
	uploadedName string
	uploaded     []byte
	deleted      string
	err          error
}

func (bucket *fakeObjectBucket) Put(_ context.Context, name string, source io.Reader, contentType string, metadata map[string]string) error {
	bucket.uploadedName = name
	bucket.uploaded, _ = io.ReadAll(source)
	if contentType != "image/png" && contentType != "image/jpeg" {
		return errors.New("unexpected content type")
	}
	if metadata["firebaseStorageDownloadTokens"] == "" {
		return errors.New("missing token")
	}
	return bucket.err
}
func (bucket *fakeObjectBucket) Delete(_ context.Context, name string) error {
	bucket.deleted = name
	return bucket.err
}

func validPNG(t *testing.T) []byte {
	t.Helper()
	var output bytes.Buffer
	picture := image.NewRGBA(image.Rect(0, 0, 1, 1))
	picture.Set(0, 0, color.RGBA{R: 1, A: 255})
	if err := png.Encode(&output, picture); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestFirebaseImageStoreValidatesAndBuildsDownloadTokenURL(t *testing.T) {
	bucket := &fakeObjectBucket{}
	store := NewFirebaseImageStore("dikasa.firebasestorage.app", bucket)
	content := validPNG(t)
	stored, err := store.Save(context.Background(), "outlet-a", "product-a", &ImageUpload{
		Reader: bytes.NewReader(content), Size: int64(len(content)), ContentType: "image/png", Filename: "menu.png",
	})
	if err != nil {
		t.Fatalf("save image: %v", err)
	}
	if !strings.HasPrefix(bucket.uploadedName, "outlets/outlet-a/products/product-a/") {
		t.Fatalf("unexpected object: %s", bucket.uploadedName)
	}
	if !strings.Contains(stored.URL, "https://firebasestorage.googleapis.com/v0/b/dikasa.firebasestorage.app/o/") || !strings.Contains(stored.URL, "token=") {
		t.Fatalf("unexpected URL: %s", stored.URL)
	}
}

func TestFirebaseImageStoreRejectsInvalidAndOversizedFiles(t *testing.T) {
	store := NewFirebaseImageStore("bucket", &fakeObjectBucket{})
	if _, err := store.Save(context.Background(), "o", "p", &ImageUpload{Reader: bytes.NewReader([]byte("not an image")), Size: 12, ContentType: "image/png", Filename: "bad.png"}); !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("expected invalid image, got %v", err)
	}
	if _, err := store.Save(context.Background(), "o", "p", &ImageUpload{Reader: bytes.NewReader(nil), Size: maxImageBytes + 1, ContentType: "image/png", Filename: "big.png"}); !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("expected too large, got %v", err)
	}
}

func TestFirebaseImageStoreMapsBucketFailures(t *testing.T) {
	bucket := &fakeObjectBucket{err: errors.New("firebase unavailable")}
	store := NewFirebaseImageStore("bucket", bucket)
	content := validPNG(t)
	_, err := store.Save(context.Background(), "o", "p", &ImageUpload{Reader: bytes.NewReader(content), Size: int64(len(content)), ContentType: "image/png", Filename: "ok.png"})
	if !errors.Is(err, ErrStorage) {
		t.Fatalf("expected storage error, got %v", err)
	}
}
