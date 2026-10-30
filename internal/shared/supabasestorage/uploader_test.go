package supabasestorage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUploaderCreatesSignedUploadAndDownloadsBoundedObject(t *testing.T) {
	objectName := "outlets/11111111-1111-4111-8111-111111111111/users/22222222-2222-4222-8222-222222222222/product-images/33333333-3333-4333-8333-333333333333.png"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("apikey") != "secret-key" || request.Header.Get("Authorization") != "Bearer secret-key" {
			t.Fatalf("missing server credentials: %#v", request.Header)
		}
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/storage/v1/object/upload/sign/dikasa-images/"+objectName:
			if request.Header.Get("x-upsert") != "" {
				t.Fatalf("signed upload unexpectedly permits overwrite: %#v", request.Header)
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"url":"/object/upload/sign/dikasa-images/` + objectName + `?token=signed-token"}`))
		case request.Method == http.MethodGet && request.URL.Path == "/storage/v1/object/dikasa-images/"+objectName:
			writer.Header().Set("Content-Type", "image/png")
			_, _ = writer.Write([]byte("png"))
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
	}))
	defer server.Close()

	uploader, err := NewUploader(server.URL, "dikasa-images", "secret-key", WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	intent, err := uploader.CreateSignedUpload(context.Background(), objectName)
	if err != nil {
		t.Fatal(err)
	}
	wantUploadURL := server.URL + "/storage/v1/object/upload/sign/dikasa-images/" + objectName + "?token=signed-token"
	if intent.ObjectName != objectName || intent.UploadURL != wantUploadURL || intent.ExpiresIn != 2*60*60 {
		t.Fatalf("unexpected signed upload: %+v", intent)
	}
	if bytes.Contains([]byte(intent.UploadURL), []byte("secret-key")) {
		t.Fatal("server credential leaked through signed upload URL")
	}
	object, err := uploader.Download(context.Background(), objectName, 3*1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(object.Body)
	if err != nil {
		t.Fatal(err)
	}
	_ = object.Body.Close()
	if object.ContentType != "image/png" || object.Size != 3 || !bytes.Equal(content, []byte("png")) {
		t.Fatalf("unexpected downloaded object: type=%q size=%d body=%q", object.ContentType, object.Size, content)
	}
}

func TestUploaderRejectsDownloadedObjectBeyondLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "image/png")
		writer.Header().Set("Content-Length", "4")
		_, _ = writer.Write([]byte("1234"))
	}))
	defer server.Close()
	uploader, err := NewUploader(server.URL, "dikasa-images", "secret-key", WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = uploader.Download(context.Background(), "outlets/a/image.png", 3); !errors.Is(err, ErrObjectTooLarge) {
		t.Fatalf("expected bounded download rejection, got %v", err)
	}
}

func TestUploaderBuildsPublicObjectAndDeletesTrustedURL(t *testing.T) {
	objectName := "outlets/11111111-1111-4111-8111-111111111111/products/22222222-2222-4222-8222-222222222222/images/33333333-3333-4333-8333-333333333333.png"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("apikey") != "secret-key" || request.Header.Get("Authorization") != "Bearer secret-key" {
			t.Fatalf("missing server credentials: %#v", request.Header)
		}
		wantPath := "/storage/v1/object/dikasa-images/" + objectName
		if request.URL.Path != wantPath {
			t.Fatalf("unexpected path %q, want %q", request.URL.Path, wantPath)
		}
		switch request.Method {
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
	publicURL, err := uploader.PublicURL(objectName)
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
