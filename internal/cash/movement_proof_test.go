package cash

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"be-dikasa/internal/shared/imagefile"
	"be-dikasa/internal/shared/supabasestorage"
)

func TestSupabaseMovementProofStoreUsesSharedPublicURL(t *testing.T) {
	var uploaded, deleted string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		name := strings.TrimPrefix(request.URL.Path, "/storage/v1/object/dikasa-images/")
		if request.Method == http.MethodDelete {
			deleted = name
		} else {
			uploaded = name
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	uploader, err := supabasestorage.NewUploader(server.URL, "dikasa-images", "secret", supabasestorage.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	if err = png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	validated, err := imagefile.Validate(imagefile.Upload{Reader: bytes.NewReader(encoded.Bytes()), Size: int64(encoded.Len()), ContentType: "image/png", Filename: "proof.png"}, imagefile.StandardPolicy)
	if err != nil {
		t.Fatal(err)
	}
	store := NewSupabaseMovementProofStore(uploader)
	object, err := store.Save(context.Background(), "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", validated)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(uploaded, "outlets/11111111-1111-4111-8111-111111111111/cash-sessions/22222222-2222-4222-8222-222222222222/proofs/") || !strings.Contains(object.URL, "/storage/v1/object/public/dikasa-images/") {
		t.Fatalf("unexpected proof object=%q url=%q", uploaded, object.URL)
	}
	if err = store.Delete(context.Background(), object.URL); err != nil || deleted != uploaded {
		t.Fatalf("delete failed: key=%q error=%v", deleted, err)
	}
}
