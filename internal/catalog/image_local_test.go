package catalog

import (
	"bytes"
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalImagesSaveServeDeleteAndRejectPaths(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalImageStore(dir, "http://127.0.0.1:8081")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	data := validPNG(t)
	image, err := store.Save(context.Background(), "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", &ImageUpload{Reader: bytes.NewReader(data), Size: int64(len(data)), ContentType: "image/png", Filename: "photo.png"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(image.URL, "http://127.0.0.1:8081/dikasa/v1/product-images/") {
		t.Fatal(image.URL)
	}
	r := httptest.NewRecorder()
	store.ServeHTTP(r, httptest.NewRequest("GET", image.URL, nil))
	if r.Code != 200 || !bytes.Equal(r.Body.Bytes(), data) {
		t.Fatalf("serve: %d", r.Code)
	}
	for _, name := range []string{"../secret.png", "secret.png", "/etc/passwd", `..\secret.png`} {
		if err := store.Delete(context.Background(), name); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	if err := store.Delete(context.Background(), image.ObjectName); err != nil {
		t.Fatal(err)
	}
	r = httptest.NewRecorder()
	store.ServeHTTP(r, httptest.NewRequest("GET", image.URL, nil))
	if r.Code != 404 {
		t.Fatal(r.Code)
	}
	if _, err = store.Save(context.Background(), "../escape", "p", &ImageUpload{}); !errors.Is(err, ErrInvalidImage) {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(dir, image.ObjectName)); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestLocalImageRejectsSymlinkDirectory(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(root, "link")
	if err := os.Symlink(t.TempDir(), link); err != nil {
		t.Skip("symlink creation unavailable")
	}
	if store, err := NewLocalImageStore(link, "http://localhost:8081"); err == nil {
		store.Close()
		t.Fatal("accepted symlink root")
	}
}
