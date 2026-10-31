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

type catalogStorageStub struct {
	object  string
	deleted string
	content []byte
	status  int
}

func newCatalogImageStore(t *testing.T, stub *catalogStorageStub) *SupabaseImageStore {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		name := strings.TrimPrefix(request.URL.Path, "/storage/v1/object/dikasa-images/")
		switch {
		case request.Method == http.MethodPost && strings.HasPrefix(request.URL.Path, "/storage/v1/object/upload/sign/dikasa-images/"):
			stub.object = strings.TrimPrefix(request.URL.Path, "/storage/v1/object/upload/sign/dikasa-images/")
			if stub.status != 0 {
				writer.WriteHeader(stub.status)
				return
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"url":"/object/upload/sign/dikasa-images/` + stub.object + `?token=product-token"}`))
		case request.Method == http.MethodGet:
			stub.object = name
			if stub.status != 0 {
				writer.WriteHeader(stub.status)
				return
			}
			writer.Header().Set("Content-Type", "image/png")
			_, _ = writer.Write(stub.content)
		case request.Method == http.MethodDelete:
			stub.deleted = name
			writer.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected storage request: %s %s", request.Method, request.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	uploader, err := supabasestorage.NewUploader(server.URL, "dikasa-images", "secret", supabasestorage.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	return NewSupabaseImageStore(uploader)
}

func TestSupabaseImageStorePreparesAndAdoptsDirectUpload(t *testing.T) {
	stub := &catalogStorageStub{content: validPNG(t)}
	store := newCatalogImageStore(t, stub)
	outletID, actorID := "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	intent, err := store.Prepare(context.Background(), outletID, actorID, "image/png", int64(len(stub.content)))
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := "outlets/" + outletID + "/users/" + actorID + "/product-images/"
	if !strings.HasPrefix(intent.ObjectName, wantPrefix) || !strings.Contains(intent.UploadURL, "token=product-token") {
		t.Fatalf("unexpected intent: %+v", intent)
	}
	stored, err := store.Adopt(context.Background(), outletID, actorID, intent.ObjectName)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stored.URL, "/storage/v1/object/public/dikasa-images/"+intent.ObjectName) {
		t.Fatalf("unexpected public URL: %s", stored.URL)
	}
}

func TestSupabaseImageStoreRejectsInvalidIntentAndTamperedCompletion(t *testing.T) {
	stub := &catalogStorageStub{content: []byte("not an image")}
	store := newCatalogImageStore(t, stub)
	outletID, actorID := "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	if _, err := store.Prepare(context.Background(), outletID, actorID, "image/gif", 10); !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("expected invalid type, got %v", err)
	}
	if _, err := store.Prepare(context.Background(), outletID, actorID, "image/png", maxImageBytes+1); !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("expected oversized intent, got %v", err)
	}
	foreign := "outlets/" + outletID + "/users/33333333-3333-4333-8333-333333333333/product-images/44444444-4444-4444-8444-444444444444.png"
	if _, err := store.Adopt(context.Background(), outletID, actorID, foreign); !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("expected actor-bound path rejection, got %v", err)
	}
	intent, err := store.Prepare(context.Background(), outletID, actorID, "image/png", 12)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Adopt(context.Background(), outletID, actorID, intent.ObjectName); !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("expected invalid stored bytes, got %v", err)
	}
	if stub.deleted != intent.ObjectName {
		t.Fatalf("invalid object was not deleted: %q", stub.deleted)
	}
}

func TestSupabaseImageStoreMapsSigningFailure(t *testing.T) {
	store := newCatalogImageStore(t, &catalogStorageStub{status: http.StatusInternalServerError})
	_, err := store.Prepare(context.Background(), "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", "image/png", 100)
	if !errors.Is(err, ErrStorage) {
		t.Fatalf("expected storage error, got %v", err)
	}
}
