package catalog

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

const maxImageBytes int64 = 3 * 1024 * 1024

// ObjectBucket adalah operasi object storage minimum agar adapter Firebase dapat diuji tanpa jaringan.
type ObjectBucket interface {
	Put(context.Context, string, io.Reader, string, map[string]string) error
	Delete(context.Context, string) error
}

// FirebaseImageStore menyimpan gambar dengan metadata download token Firebase.
type FirebaseImageStore struct {
	bucketName string
	bucket     ObjectBucket
}

// NewFirebaseImageStore membuat penyimpanan gambar pada bucket yang sudah siap.
func NewFirebaseImageStore(bucketName string, bucket ObjectBucket) *FirebaseImageStore {
	return &FirebaseImageStore{bucketName: strings.TrimSpace(bucketName), bucket: bucket}
}

// Save memvalidasi ukuran, MIME, signature, extension, dan decode sebelum upload.
func (store *FirebaseImageStore) Save(ctx context.Context, outletID, productID string, upload *ImageUpload) (*StoredImage, error) {
	content, extension, declared, err := validateImage(upload)
	if err != nil {
		return nil, err
	}
	token := uuid.NewString()
	objectName := fmt.Sprintf("outlets/%s/products/%s/%s%s", outletID, productID, uuid.NewString(), extension)
	if err = store.bucket.Put(ctx, objectName, bytes.NewReader(content), declared, map[string]string{"firebaseStorageDownloadTokens": token}); err != nil {
		return nil, fmt.Errorf("%w: upload object", ErrStorage)
	}
	downloadURL := fmt.Sprintf("https://firebasestorage.googleapis.com/v0/b/%s/o/%s?alt=media&token=%s", url.PathEscape(store.bucketName), url.PathEscape(objectName), url.QueryEscape(token))
	return &StoredImage{URL: downloadURL, ObjectName: objectName}, nil
}

// Delete menghapus object yang tidak lagi direferensikan produk.
func (store *FirebaseImageStore) Delete(ctx context.Context, objectName string) error {
	if strings.TrimSpace(objectName) == "" {
		return nil
	}
	if err := store.bucket.Delete(ctx, objectName); err != nil {
		return fmt.Errorf("%w: delete object", ErrStorage)
	}
	return nil
}

type cloudBucket struct{ handle *storage.BucketHandle }

func (bucket cloudBucket) Put(ctx context.Context, name string, source io.Reader, contentType string, metadata map[string]string) error {
	writer := bucket.handle.Object(name).NewWriter(ctx)
	writer.ContentType = contentType
	writer.Metadata = metadata
	if _, err := io.Copy(writer, source); err != nil {
		_ = writer.CloseWithError(err)
		return err
	}
	return writer.Close()
}

func (bucket cloudBucket) Delete(ctx context.Context, name string) error {
	return bucket.handle.Object(name).Delete(ctx)
}

// NewCloudFirebaseImageStore membuka bucket melalui Application Default Credentials dan memverifikasinya saat startup.
func NewCloudFirebaseImageStore(ctx context.Context, bucketName string) (*FirebaseImageStore, func() error, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("inisialisasi Firebase Storage: %w", err)
	}
	bucket := client.Bucket(bucketName)
	if _, err = bucket.Attrs(ctx); err != nil {
		_ = client.Close()
		return nil, nil, fmt.Errorf("verifikasi Firebase Storage bucket: %w", err)
	}
	return NewFirebaseImageStore(bucketName, cloudBucket{handle: bucket}), client.Close, nil
}
