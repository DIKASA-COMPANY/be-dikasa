package cash

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"be-dikasa/internal/shared/supabasestorage"
)

type proofStorageStub struct {
	object  string
	deleted string
	content []byte
}

func newProofStore(t *testing.T, stub *proofStorageStub) *SupabaseMovementProofStore {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodPost && strings.HasPrefix(request.URL.Path, "/storage/v1/object/upload/sign/dikasa-images/"):
			stub.object = strings.TrimPrefix(request.URL.Path, "/storage/v1/object/upload/sign/dikasa-images/")
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"url":"/object/upload/sign/dikasa-images/` + stub.object + `?token=proof-token"}`))
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
	return NewSupabaseMovementProofStore(uploader)
}

func TestSupabaseMovementProofStorePreparesAndAdoptsDirectUpload(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	stub := &proofStorageStub{content: encoded.Bytes()}
	store := newProofStore(t, stub)
	outletID := "11111111-1111-4111-8111-111111111111"
	sessionID := "22222222-2222-4222-8222-222222222222"
	actorID := "33333333-3333-4333-8333-333333333333"
	requestID := "44444444-4444-4444-8444-444444444444"
	intent, err := store.Prepare(context.Background(), outletID, sessionID, actorID, requestID, "image/png", int64(encoded.Len()))
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := "outlets/" + outletID + "/cash-sessions/" + sessionID + "/users/" + actorID + "/requests/" + requestID + "/proofs/"
	if !strings.HasPrefix(intent.ObjectName, wantPrefix) || !strings.Contains(intent.UploadURL, "token=proof-token") {
		t.Fatalf("unexpected proof intent: %+v", intent)
	}
	object, err := store.Adopt(context.Background(), outletID, sessionID, actorID, requestID, intent.ObjectName)
	if err != nil || !strings.Contains(object.URL, "/storage/v1/object/public/dikasa-images/"+intent.ObjectName) {
		t.Fatalf("unexpected adopted proof: %+v error=%v", object, err)
	}
}

func TestSupabaseMovementProofStoreRejectsTamperedOrInvalidCompletion(t *testing.T) {
	stub := &proofStorageStub{content: []byte("bad")}
	store := newProofStore(t, stub)
	outletID := "11111111-1111-4111-8111-111111111111"
	sessionID := "22222222-2222-4222-8222-222222222222"
	actorID := "33333333-3333-4333-8333-333333333333"
	requestID := "44444444-4444-4444-8444-444444444444"
	foreign := "outlets/" + outletID + "/cash-sessions/" + sessionID + "/users/55555555-5555-4555-8555-555555555555/requests/" + requestID + "/proofs/66666666-6666-4666-8666-666666666666.png"
	if _, err := store.Adopt(context.Background(), outletID, sessionID, actorID, requestID, foreign); !errors.Is(err, ErrProofInvalid) {
		t.Fatalf("expected actor-bound proof rejection, got %v", err)
	}
	intent, err := store.Prepare(context.Background(), outletID, sessionID, actorID, requestID, "image/png", 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Adopt(context.Background(), outletID, sessionID, actorID, requestID, intent.ObjectName); !errors.Is(err, ErrProofInvalid) {
		t.Fatalf("expected invalid stored proof, got %v", err)
	}
	if stub.deleted != intent.ObjectName {
		t.Fatalf("invalid proof was not deleted: %q", stub.deleted)
	}
	if _, err = store.Prepare(context.Background(), outletID, sessionID, actorID, requestID, "image/png", maxProofBytes+1); !errors.Is(err, ErrProofTooLarge) {
		t.Fatalf("expected oversized proof intent, got %v", err)
	}
}
