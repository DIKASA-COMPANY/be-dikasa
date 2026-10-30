package catalog

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"be-dikasa/internal/shared/supabasestorage"
)

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

func newCatalogImageStore(t *testing.T, status int, uploaded *string) *SupabaseImageStore {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if uploaded != nil {
			*uploaded = strings.TrimPrefix(request.URL.Path, "/storage/v1/object/dikasa-images/")
		}
		writer.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	uploader, err := supabasestorage.NewUploader(server.URL, "dikasa-images", "secret", supabasestorage.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	return NewSupabaseImageStore(uploader)
}

func TestSupabaseImageStoreValidatesAndBuildsPublicURL(t *testing.T) {
	var uploaded string
	store := newCatalogImageStore(t, http.StatusOK, &uploaded)
	content := validPNG(t)
	stored, err := store.Save(context.Background(), "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", &ImageUpload{
		Reader: bytes.NewReader(content), Size: int64(len(content)), ContentType: "image/png", Filename: "menu.png",
	})
	if err != nil {
		t.Fatalf("save image: %v", err)
	}
	if !strings.HasPrefix(uploaded, "outlets/11111111-1111-4111-8111-111111111111/products/22222222-2222-4222-8222-222222222222/images/") {
		t.Fatalf("unexpected object: %s", uploaded)
	}
	if !strings.Contains(stored.URL, "/storage/v1/object/public/dikasa-images/") {
		t.Fatalf("unexpected URL: %s", stored.URL)
	}
}

func TestSupabaseImageStoreRejectsInvalidAndOversizedFiles(t *testing.T) {
	store := newCatalogImageStore(t, http.StatusOK, nil)
	outletID, productID := "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	if _, err := store.Save(context.Background(), outletID, productID, &ImageUpload{Reader: bytes.NewReader([]byte("not an image")), Size: 12, ContentType: "image/png", Filename: "bad.png"}); !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("expected invalid image, got %v", err)
	}
	if _, err := store.Save(context.Background(), outletID, productID, &ImageUpload{Reader: bytes.NewReader(nil), Size: maxImageBytes + 1, ContentType: "image/png", Filename: "big.png"}); !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("expected too large, got %v", err)
	}
}

func TestSupabaseImageStoreMapsStorageFailures(t *testing.T) {
	store := newCatalogImageStore(t, http.StatusInternalServerError, nil)
	content := validPNG(t)
	_, err := store.Save(context.Background(), "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", &ImageUpload{Reader: bytes.NewReader(content), Size: int64(len(content)), ContentType: "image/png", Filename: "ok.png"})
	if !errors.Is(err, ErrStorage) {
		t.Fatalf("expected storage error, got %v", err)
	}
}

func TestSupabaseImageStoreRejectsUnsafeIdentifiersAndURLs(t *testing.T) {
	content := validPNG(t)
	store := newCatalogImageStore(t, http.StatusOK, nil)
	_, err := store.Save(context.Background(), "../other-outlet", "22222222-2222-4222-8222-222222222222", &ImageUpload{Reader: bytes.NewReader(content), Size: int64(len(content)), ContentType: "image/png", Filename: "ok.png"})
	if !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("expected invalid identifier rejection, got %v", err)
	}
	if err = store.Delete(context.Background(), "https://example.com/outlets/not-a-uuid/secrets/config.json"); !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("expected unsafe delete rejection, got %v", err)
	}
}
