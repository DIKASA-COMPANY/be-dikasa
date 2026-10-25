package user

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
)

const maxAvatarBytes int64 = 3 * 1024 * 1024

// AvatarObjectBucket adalah operasi bucket minimum agar Firebase avatar dapat diuji tanpa jaringan.
type AvatarObjectBucket interface {
	Put(context.Context, string, io.Reader, string, map[string]string) error
	Delete(context.Context, string) error
}

// FirebaseAvatarStore menyimpan avatar dengan Firebase download token.
type FirebaseAvatarStore struct {
	bucketName string
	bucket     AvatarObjectBucket
}

// NewFirebaseAvatarStore membuat storage avatar pada bucket yang siap dipakai.
func NewFirebaseAvatarStore(bucketName string, bucket AvatarObjectBucket) *FirebaseAvatarStore {
	return &FirebaseAvatarStore{bucketName: strings.TrimSpace(bucketName), bucket: bucket}
}

// Save memvalidasi MIME, signature, extension, decode, dan batas tiga MiB sebelum upload.
func (store *FirebaseAvatarStore) Save(ctx context.Context, outletID, userID string, upload *AvatarUpload) (*AvatarImage, error) {
	content, extension, declared, err := validateAvatar(upload)
	if err != nil {
		return nil, err
	}
	token := uuid.NewString()
	objectName := fmt.Sprintf("outlets/%s/users/%s/avatar/%s%s", outletID, userID, uuid.NewString(), extension)
	if err = store.bucket.Put(ctx, objectName, bytes.NewReader(content), declared, map[string]string{"firebaseStorageDownloadTokens": token}); err != nil {
		return nil, fmt.Errorf("%w: upload", ErrAvatarStorage)
	}
	downloadURL := fmt.Sprintf("https://firebasestorage.googleapis.com/v0/b/%s/o/%s?alt=media&token=%s", url.PathEscape(store.bucketName), url.PathEscape(objectName), url.QueryEscape(token))
	return &AvatarImage{URL: downloadURL, ObjectName: objectName}, nil
}

// Delete menghapus avatar yang sudah tidak direferensikan.
func (store *FirebaseAvatarStore) Delete(ctx context.Context, objectName string) error {
	if strings.TrimSpace(objectName) == "" {
		return nil
	}
	if !strings.HasPrefix(objectName, "outlets/") || !strings.Contains(objectName, "/avatar/") {
		return ErrInvalidAvatar
	}
	if err := store.bucket.Delete(ctx, objectName); err != nil {
		return fmt.Errorf("%w: delete", ErrAvatarStorage)
	}
	return nil
}

type cloudAvatarBucket struct{ handle *storage.BucketHandle }

func (bucket cloudAvatarBucket) Put(ctx context.Context, name string, source io.Reader, contentType string, metadata map[string]string) error {
	writer := bucket.handle.Object(name).NewWriter(ctx)
	writer.ContentType, writer.Metadata = contentType, metadata
	if _, err := io.Copy(writer, source); err != nil {
		_ = writer.CloseWithError(err)
		return err
	}
	return writer.Close()
}
func (bucket cloudAvatarBucket) Delete(ctx context.Context, name string) error {
	return bucket.handle.Object(name).Delete(ctx)
}

// NewCloudFirebaseAvatarStore membuka dan memverifikasi bucket menggunakan Application Default Credentials.
func NewCloudFirebaseAvatarStore(ctx context.Context, bucketName string) (*FirebaseAvatarStore, func() error, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("inisialisasi avatar Firebase Storage: %w", err)
	}
	bucket := client.Bucket(bucketName)
	if _, err = bucket.Attrs(ctx); err != nil {
		_ = client.Close()
		return nil, nil, fmt.Errorf("verifikasi bucket avatar Firebase Storage: %w", err)
	}
	return NewFirebaseAvatarStore(bucketName, cloudAvatarBucket{handle: bucket}), client.Close, nil
}
