package catalog

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// LocalImageStore confines product photos to one persistent directory.
type LocalImageStore struct {
	root    *os.Root
	baseURL string
}

// NewLocalImageStore rejects symlink roots and opens a traversal-resistant filesystem root.
func NewLocalImageStore(directory, baseURL string) (*LocalImageStore, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(absolute, 0700); err != nil {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(filepath.Clean(absolute), filepath.Clean(resolved)) {
		return nil, fmt.Errorf("image directory must not contain symlinks")
	}
	root, err := os.OpenRoot(absolute)
	if err != nil {
		return nil, err
	}
	return &LocalImageStore{root: root, baseURL: strings.TrimRight(baseURL, "/")}, nil
}

// Close releases the filesystem root after the HTTP server stops.
func (s *LocalImageStore) Close() error { return s.root.Close() }

// Save validates content and stores it under an exclusively created random name.
func (s *LocalImageStore) Save(_ context.Context, outletID, productID string, upload *ImageUpload) (*StoredImage, error) {
	if _, err := uuid.Parse(outletID); err != nil {
		return nil, ErrInvalidImage
	}
	if _, err := uuid.Parse(productID); err != nil {
		return nil, ErrInvalidImage
	}
	data, extension, _, err := validateImage(upload)
	if err != nil {
		return nil, err
	}
	name := uuid.MustParse(outletID).String() + "_" + uuid.MustParse(productID).String() + "_" + uuid.NewString() + extension
	file, err := s.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, fmt.Errorf("%w: create image", ErrStorage)
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = s.root.Remove(name)
		return nil, fmt.Errorf("%w: write image", ErrStorage)
	}
	return &StoredImage{URL: s.baseURL + "/dikasa/v1/product-images/" + name, ObjectName: name}, nil
}

// validLocalImageName admits only three canonical UUID components and a trusted image extension.
func validLocalImageName(name string) bool {
	extension := filepath.Ext(name)
	if extension != ".png" && extension != ".jpg" {
		return false
	}
	parts := strings.Split(strings.TrimSuffix(name, extension), "_")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		id, err := uuid.Parse(part)
		if err != nil || id.String() != part {
			return false
		}
	}
	return true
}

// Delete accepts only generated filenames and never follows symlinks.
func (s *LocalImageStore) Delete(_ context.Context, name string) error {
	if !validLocalImageName(name) {
		return ErrInvalidImage
	}
	info, err := s.root.Lstat(name)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return ErrStorage
	}
	if !info.Mode().IsRegular() {
		return ErrInvalidImage
	}
	return s.root.Remove(name)
}

// ServeHTTP exposes only regular generated images, without directory listings or filesystem paths.
func (s *LocalImageStore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/dikasa/v1/product-images/")
	if (r.Method != "GET" && r.Method != "HEAD") || !validLocalImageName(name) {
		http.NotFound(w, r)
		return
	}
	info, err := s.root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	file, err := s.root.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	if strings.HasSuffix(name, ".png") {
		w.Header().Set("Content-Type", "image/png")
	} else {
		w.Header().Set("Content-Type", "image/jpeg")
	}
	http.ServeContent(w, r, name, info.ModTime(), file)
}
