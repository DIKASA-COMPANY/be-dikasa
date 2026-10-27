package catalog

import (
	"be-dikasa/internal/shared/firebaseimage"
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/google/uuid"
)

const maxImageBytes int64 = 3 * 1024 * 1024

// FirebaseImageStore stores product images through the shared Firebase uploader.
type FirebaseImageStore struct{ uploader *firebaseimage.Uploader }

// NewFirebaseImageStore creates product image storage using the shared upload mechanism.
func NewFirebaseImageStore(uploader *firebaseimage.Uploader) *FirebaseImageStore {
	return &FirebaseImageStore{uploader: uploader}
}

// Save validates an image, uploads it to Firebase, and returns only its download URL.
func (store *FirebaseImageStore) Save(ctx context.Context, outletID, productID string, upload *ImageUpload) (*StoredImage, error) {
	outlet, err := uuid.Parse(outletID)
	if err != nil || outlet == uuid.Nil || outlet.String() != outletID {
		return nil, ErrInvalidImage
	}
	product, err := uuid.Parse(productID)
	if err != nil || product == uuid.Nil || product.String() != productID {
		return nil, ErrInvalidImage
	}
	content, extension, declared, err := validateImage(upload)
	if err != nil {
		return nil, err
	}
	objectName := fmt.Sprintf("outlets/%s/products/%s/images/%s%s", outlet, product, uuid.NewString(), extension)
	downloadURL, err := store.uploader.Upload(ctx, objectName, bytes.NewReader(content), declared)
	if err != nil {
		return nil, fmt.Errorf("%w: upload object", ErrStorage)
	}
	return &StoredImage{URL: downloadURL}, nil
}

// Delete removes a product image identified by its persisted Firebase download URL.
func (store *FirebaseImageStore) Delete(ctx context.Context, downloadURL string) error {
	if strings.TrimSpace(downloadURL) == "" {
		return nil
	}
	objectName, err := store.uploader.ObjectName(downloadURL)
	if err != nil || !validFirebaseProductImageKey(objectName) {
		return ErrInvalidImage
	}
	if err = store.uploader.DeleteURL(ctx, downloadURL); err != nil {
		return fmt.Errorf("%w: delete object", ErrStorage)
	}
	return nil
}

// validFirebaseProductImageKey restricts deletion to server-generated product images.
func validFirebaseProductImageKey(objectName string) bool {
	parts := strings.Split(objectName, "/")
	if len(parts) != 6 || parts[0] != "outlets" || parts[2] != "products" || parts[4] != "images" {
		return false
	}
	for _, index := range []int{1, 3} {
		id, err := uuid.Parse(parts[index])
		if err != nil || id == uuid.Nil || id.String() != parts[index] {
			return false
		}
	}
	extension := path.Ext(parts[5])
	if extension != ".png" && extension != ".jpg" {
		return false
	}
	identifier := strings.TrimSuffix(parts[5], extension)
	id, err := uuid.Parse(identifier)
	return err == nil && id != uuid.Nil && id.String() == identifier
}
