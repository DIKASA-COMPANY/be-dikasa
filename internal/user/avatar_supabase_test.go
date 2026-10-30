package user

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

func newAvatarStore(t *testing.T, uploaded *string) *SupabaseAvatarStore {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if uploaded != nil {
			*uploaded = strings.TrimPrefix(request.URL.Path, "/storage/v1/object/dikasa-images/")
		}
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	uploader, err := supabasestorage.NewUploader(server.URL, "dikasa-images", "secret", supabasestorage.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	return NewSupabaseAvatarStore(uploader)
}

func TestSupabaseAvatarStoreUsesOutletUserPathAndPublicURL(t *testing.T) {
	var uploaded string
	store := newAvatarStore(t, &uploaded)
	content := avatarPNG(t)
	stored, err := store.Save(context.Background(), "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", &AvatarUpload{Reader: bytes.NewReader(content), Size: int64(len(content)), ContentType: "image/png", Filename: "avatar.png"})
	if err != nil {
		t.Fatalf("save avatar: %v", err)
	}
	if !strings.HasPrefix(uploaded, "outlets/11111111-1111-4111-8111-111111111111/users/22222222-2222-4222-8222-222222222222/avatar/") || !strings.Contains(stored.URL, "/storage/v1/object/public/dikasa-images/") {
		t.Fatalf("unexpected avatar object=%q stored=%+v", uploaded, stored)
	}
}

func TestSupabaseAvatarStoreRejectsUnsafeObjectIdentifiers(t *testing.T) {
	content := avatarPNG(t)
	store := newAvatarStore(t, nil)
	_, err := store.Save(context.Background(), "../other-outlet", "22222222-2222-4222-8222-222222222222", &AvatarUpload{Reader: bytes.NewReader(content), Size: int64(len(content)), ContentType: "image/png", Filename: "ok.png"})
	if !errors.Is(err, ErrInvalidAvatar) {
		t.Fatalf("expected invalid identifier rejection, got %v", err)
	}
	if err = store.Delete(context.Background(), "https://example.com/outlets/not-a-uuid/other/avatar/file.png"); !errors.Is(err, ErrInvalidAvatar) {
		t.Fatalf("expected unsafe delete rejection, got %v", err)
	}
}

func TestSupabaseAvatarStoreRejectsInvalidAndOversizedFiles(t *testing.T) {
	store := newAvatarStore(t, nil)
	outletID, userID := "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	if _, err := store.Save(context.Background(), outletID, userID, &AvatarUpload{Reader: bytes.NewReader([]byte("bad")), Size: 3, ContentType: "image/png", Filename: "bad.png"}); !errors.Is(err, ErrInvalidAvatar) {
		t.Fatalf("invalid avatar: %v", err)
	}
	if _, err := store.Save(context.Background(), outletID, userID, &AvatarUpload{Reader: bytes.NewReader(nil), Size: maxAvatarBytes + 1, ContentType: "image/png", Filename: "large.png"}); !errors.Is(err, ErrAvatarTooLarge) {
		t.Fatalf("large avatar: %v", err)
	}
}
