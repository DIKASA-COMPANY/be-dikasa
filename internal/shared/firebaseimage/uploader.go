// Package firebaseimage provides the single Firebase upload and download-URL mechanism used by image APIs.
package firebaseimage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
)

var (
	// ErrInvalidURL indicates a URL that is not a tokenized download URL for the configured bucket.
	ErrInvalidURL = errors.New("invalid Firebase download URL")
	// ErrStorage indicates an underlying Firebase Storage operation failure.
	ErrStorage = errors.New("Firebase Storage operation failed")
)

// Bucket is the minimum object-storage behavior required by Uploader.
type Bucket interface {
	Put(context.Context, string, io.Reader, string, map[string]string) error
	Delete(context.Context, string) error
}

// Uploader uploads validated image bytes and returns Firebase download URLs.
type Uploader struct {
	bucketName string
	bucket     Bucket
}

// NewUploader creates a Firebase URL uploader over an initialized bucket.
func NewUploader(bucketName string, bucket Bucket) *Uploader {
	return &Uploader{bucketName: strings.TrimSpace(bucketName), bucket: bucket}
}

// Upload writes one image object with a random download token and returns its renderable URL.
func (uploader *Uploader) Upload(ctx context.Context, objectName string, source io.Reader, contentType string) (string, error) {
	token := uuid.NewString()
	if err := uploader.bucket.Put(ctx, objectName, source, contentType, map[string]string{"firebaseStorageDownloadTokens": token}); err != nil {
		return "", fmt.Errorf("%w: upload object", ErrStorage)
	}
	return fmt.Sprintf("https://firebasestorage.googleapis.com/v0/b/%s/o/%s?alt=media&token=%s", url.PathEscape(uploader.bucketName), url.PathEscape(objectName), url.QueryEscape(token)), nil
}

// ObjectName extracts a trusted object name from this uploader's Firebase download URL.
func (uploader *Uploader) ObjectName(downloadURL string) (string, error) {
	parsed, err := url.Parse(downloadURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "firebasestorage.googleapis.com" || parsed.User != nil || parsed.Fragment != "" {
		return "", ErrInvalidURL
	}
	prefix := "/v0/b/" + url.PathEscape(uploader.bucketName) + "/o/"
	escapedPath := parsed.EscapedPath()
	if !strings.HasPrefix(escapedPath, prefix) || parsed.Query().Get("alt") != "media" || parsed.Query().Get("token") == "" {
		return "", ErrInvalidURL
	}
	objectName, err := url.PathUnescape(strings.TrimPrefix(escapedPath, prefix))
	if err != nil || objectName == "" || strings.ContainsRune(objectName, '\x00') {
		return "", ErrInvalidURL
	}
	return objectName, nil
}

// DeleteURL deletes the object identified by a trusted URL from the configured bucket.
func (uploader *Uploader) DeleteURL(ctx context.Context, downloadURL string) error {
	objectName, err := uploader.ObjectName(downloadURL)
	if err != nil {
		return err
	}
	if err = uploader.bucket.Delete(ctx, objectName); err != nil {
		return fmt.Errorf("%w: delete object", ErrStorage)
	}
	return nil
}

type cloudBucket struct{ handle *storage.BucketHandle }

func (bucket cloudBucket) Put(ctx context.Context, name string, source io.Reader, contentType string, metadata map[string]string) error {
	writer := bucket.handle.Object(name).NewWriter(ctx)
	writer.ContentType, writer.Metadata = contentType, metadata
	if _, err := io.Copy(writer, source); err != nil {
		_ = writer.CloseWithError(err)
		return err
	}
	return writer.Close()
}

func (bucket cloudBucket) Delete(ctx context.Context, name string) error {
	return bucket.handle.Object(name).Delete(ctx)
}

// NewCloudUploader opens and verifies one Firebase Storage bucket for all image adapters.
func NewCloudUploader(ctx context.Context, bucketName string) (*Uploader, func() error, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("inisialisasi Firebase Storage: %w", err)
	}
	bucket := client.Bucket(bucketName)
	if _, err = bucket.Attrs(ctx); err != nil {
		_ = client.Close()
		return nil, nil, fmt.Errorf("verifikasi Firebase Storage bucket: %w", err)
	}
	return NewUploader(bucketName, cloudBucket{handle: bucket}), client.Close, nil
}
