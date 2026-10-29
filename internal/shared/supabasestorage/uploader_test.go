package supabasestorage

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUploaderStoresPublicObjectAndDeletesTrustedURL(t *testing.T) {
	objectName := "outlets/11111111-1111-4111-8111-111111111111/products/22222222-2222-4222-8222-222222222222/images/33333333-3333-4333-8333-333333333333.png"
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if request.Header.Get("apikey") != "secret-key" || request.Header.Get("Authorization") != "Bearer secret-key" {
			t.Fatalf("missing server credentials: %#v", request.Header)
		}
		wantPath := "/storage/v1/object/dikasa-images/" + objectName
		if request.URL.Path != wantPath {
			t.Fatalf("unexpected path %q, want %q", request.URL.Path, wantPath)
		}
		switch request.Method {
		case http.MethodPost:
			if request.Header.Get("Content-Type") != "image/png" || request.Header.Get("x-upsert") != "false" {
				t.Fatalf("unexpected upload headers: %#v", request.Header)
			}
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte(`{"Key":"dikasa-images/object.png"}`))
		case http.MethodDelete:
			writer.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected method %s", request.Method)
		}
	}))
	defer server.Close()

	uploader, err := NewUploader(server.URL, "dikasa-images", "secret-key", WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	publicURL, err := uploader.Upload(context.Background(), objectName, bytes.NewReader([]byte("png")), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	wantURL := server.URL + "/storage/v1/object/public/dikasa-images/" + objectName
	if publicURL != wantURL {
		t.Fatalf("unexpected public URL %q, want %q", publicURL, wantURL)
	}
	if err = uploader.DeleteURL(context.Background(), publicURL); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("expected upload and delete, got %d requests", requests)
	}
}

func TestUploaderRejectsForeignOrMalformedURLs(t *testing.T) {
	uploader, err := NewUploader("https://project.supabase.co", "dikasa-images", "secret-key")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{
		"https://example.com/storage/v1/object/public/dikasa-images/image.png",
		"https://project.supabase.co/storage/v1/object/public/other/image.png",
		"https://project.supabase.co/storage/v1/object/sign/dikasa-images/image.png",
	} {
		if _, err = uploader.ObjectName(value); err == nil {
			t.Fatalf("accepted untrusted URL %q", value)
		}
	}
}

func TestUploaderVerifiesPublicBucket(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/storage/v1/bucket/dikasa-images" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"dikasa-images","name":"dikasa-images","public":true}`))
	}))
	defer server.Close()
	uploader, err := NewUploader(server.URL, "dikasa-images", "secret-key", WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	if err = uploader.VerifyBucket(context.Background()); err != nil {
		t.Fatal(err)
	}
}
