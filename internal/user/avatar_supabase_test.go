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

type avatarStorageStub struct {
	object  string
	deleted string
	content []byte
}

func newAvatarStore(t *testing.T, stub *avatarStorageStub) *SupabaseAvatarStore {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodPost && strings.HasPrefix(request.URL.Path, "/storage/v1/object/upload/sign/dikasa-images/"):
			stub.object = strings.TrimPrefix(request.URL.Path, "/storage/v1/object/upload/sign/dikasa-images/")
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"url":"/object/upload/sign/dikasa-images/` + stub.object + `?token=avatar-token"}`))
		case request.Method == http.MethodGet:
			stub.object = strings.TrimPrefix(request.URL.Path, "/storage/v1/object/dikasa-images/")
			writer.Header().Set("Content-Type", "image/png")
			_, _ = writer.Write(stub.content)
		case request.Method == http.MethodDelete:
			stub.deleted = strings.TrimPrefix(request.URL.Path, "/storage/v1/object/dikasa-images/")
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
	return NewSupabaseAvatarStore(uploader)
}

func TestSupabaseAvatarStorePreparesAndAdoptsDirectUpload(t *testing.T) {
	stub := &avatarStorageStub{content: avatarPNG(t)}
	store := newAvatarStore(t, stub)
	outletID, userID := "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	intent, err := store.Prepare(context.Background(), outletID, userID, "image/png", int64(len(stub.content)))
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := "outlets/" + outletID + "/users/" + userID + "/avatar/"
	if !strings.HasPrefix(intent.ObjectName, wantPrefix) || !strings.Contains(intent.UploadURL, "token=avatar-token") {
		t.Fatalf("unexpected avatar intent: %+v", intent)
	}
	stored, err := store.Adopt(context.Background(), outletID, userID, intent.ObjectName)
	if err != nil || !strings.Contains(stored.URL, "/storage/v1/object/public/dikasa-images/"+intent.ObjectName) {
		t.Fatalf("unexpected adopted avatar: %+v error=%v", stored, err)
	}
}

func TestSupabaseAvatarStoreRejectsTamperedOrInvalidCompletion(t *testing.T) {
	stub := &avatarStorageStub{content: []byte("bad")}
	store := newAvatarStore(t, stub)
	outletID, userID := "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	foreign := "outlets/" + outletID + "/users/33333333-3333-4333-8333-333333333333/avatar/44444444-4444-4444-8444-444444444444.png"
	if _, err := store.Adopt(context.Background(), outletID, userID, foreign); !errors.Is(err, ErrInvalidAvatar) {
		t.Fatalf("expected user-bound path rejection, got %v", err)
	}
	intent, err := store.Prepare(context.Background(), outletID, userID, "image/png", 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Adopt(context.Background(), outletID, userID, intent.ObjectName); !errors.Is(err, ErrInvalidAvatar) {
		t.Fatalf("expected invalid stored avatar, got %v", err)
	}
	if stub.deleted != intent.ObjectName {
		t.Fatalf("invalid avatar was not deleted: %q", stub.deleted)
	}
	if _, err = store.Prepare(context.Background(), outletID, userID, "image/jpeg", maxAvatarBytes+1); !errors.Is(err, ErrAvatarTooLarge) {
		t.Fatalf("expected oversized avatar intent, got %v", err)
	}
}
