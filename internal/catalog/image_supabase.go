package catalog

import (
	"be-dikasa/internal/shared/imagefile"
	"be-dikasa/internal/shared/supabasestorage"
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/google/uuid"
)

const maxImageBytes int64 = 3 * 1024 * 1024

// SupabaseImageStore stores product images through the shared Supabase uploader.
type SupabaseImageStore struct{ uploader *supabasestorage.Uploader }

// NewSupabaseImageStore creates product image storage using the shared upload mechanism.
func NewSupabaseImageStore(uploader *supabasestorage.Uploader) *SupabaseImageStore {
	return &SupabaseImageStore{uploader: uploader}
}

// Prepare validates upload hints and signs one actor-bound product-image object.
func (store *SupabaseImageStore) Prepare(ctx context.Context, outletID, actorID, contentType string, size int64) (supabasestorage.SignedUpload, error) {
	outlet, err := canonicalImageUUID(outletID)
	if err != nil {
		return supabasestorage.SignedUpload{}, ErrInvalidImage
	}
	actor, err := canonicalImageUUID(actorID)
	if err != nil {
		return supabasestorage.SignedUpload{}, ErrInvalidImage
	}
	extension, err := imagefile.ValidateIntent(contentType, size, imagefile.StandardPolicy)
	if errors.Is(err, imagefile.ErrTooLarge) {
		return supabasestorage.SignedUpload{}, ErrImageTooLarge
	}
	if err != nil {
		return supabasestorage.SignedUpload{}, ErrInvalidImage
	}
	objectName := fmt.Sprintf("outlets/%s/users/%s/product-images/%s%s", outlet, actor, uuid.NewString(), extension)
	intent, err := store.uploader.CreateSignedUpload(ctx, objectName)
	if err != nil {
		return supabasestorage.SignedUpload{}, fmt.Errorf("%w: sign upload", ErrStorage)
	}
	return intent, nil
}

// Adopt validates a directly uploaded object before returning its stable public URL.
func (store *SupabaseImageStore) Adopt(ctx context.Context, outletID, actorID, objectName string) (*StoredImage, error) {
	outlet, err := canonicalImageUUID(outletID)
	if err != nil {
		return nil, ErrInvalidImage
	}
	actor, err := canonicalImageUUID(actorID)
	if err != nil || !validPreparedProductImageKey(objectName, outlet, actor) {
		return nil, ErrInvalidImage
	}
	object, err := store.uploader.Download(ctx, objectName, maxImageBytes)
	if errors.Is(err, supabasestorage.ErrObjectTooLarge) {
		store.deletePrepared(ctx, objectName)
		return nil, ErrImageTooLarge
	}
	if errors.Is(err, supabasestorage.ErrObjectNotFound) {
		return nil, ErrInvalidImage
	}
	if err != nil {
		return nil, fmt.Errorf("%w: download object", ErrStorage)
	}
	defer object.Body.Close()
	_, err = imagefile.Validate(imagefile.Upload{Reader: object.Body, Size: object.Size, ContentType: object.ContentType, Filename: path.Base(objectName)}, imagefile.StandardPolicy)
	if errors.Is(err, imagefile.ErrTooLarge) {
		store.deletePrepared(ctx, objectName)
		return nil, ErrImageTooLarge
	}
	if err != nil {
		store.deletePrepared(ctx, objectName)
		return nil, ErrInvalidImage
	}
	publicURL, err := store.uploader.PublicURL(objectName)
	if err != nil {
		return nil, fmt.Errorf("%w: public URL", ErrStorage)
	}
	return &StoredImage{URL: publicURL}, nil
}

func (store *SupabaseImageStore) deletePrepared(ctx context.Context, objectName string) {
	publicURL, err := store.uploader.PublicURL(objectName)
	if err == nil {
		_ = store.uploader.DeleteURL(ctx, publicURL)
	}
}

// Delete removes a product image identified by its persisted Supabase public URL.
func (store *SupabaseImageStore) Delete(ctx context.Context, downloadURL string) error {
	if strings.TrimSpace(downloadURL) == "" {
		return nil
	}
	objectName, err := store.uploader.ObjectName(downloadURL)
	if err != nil || (!validSupabaseProductImageKey(objectName) && !validAnyPreparedProductImageKey(objectName)) {
		return ErrInvalidImage
	}
	if err = store.uploader.DeleteURL(ctx, downloadURL); err != nil {
		return fmt.Errorf("%w: delete object", ErrStorage)
	}
	return nil
}

func canonicalImageUUID(value string) (string, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil || id.String() != value {
		return "", ErrInvalidImage
	}
	return id.String(), nil
}

func validPreparedProductImageKey(objectName, outletID, actorID string) bool {
	parts := strings.Split(objectName, "/")
	return len(parts) == 6 && parts[0] == "outlets" && parts[1] == outletID && parts[2] == "users" && parts[3] == actorID && parts[4] == "product-images" && validImageObjectFile(parts[5])
}

func validAnyPreparedProductImageKey(objectName string) bool {
	parts := strings.Split(objectName, "/")
	if len(parts) != 6 || parts[0] != "outlets" || parts[2] != "users" || parts[4] != "product-images" {
		return false
	}
	if _, err := canonicalImageUUID(parts[1]); err != nil {
		return false
	}
	if _, err := canonicalImageUUID(parts[3]); err != nil {
		return false
	}
	return validImageObjectFile(parts[5])
}

func validImageObjectFile(filename string) bool {
	extension := path.Ext(filename)
	if extension != ".png" && extension != ".jpg" {
		return false
	}
	_, err := canonicalImageUUID(strings.TrimSuffix(filename, extension))
	return err == nil
}

// validSupabaseProductImageKey restricts deletion to server-generated product images.
func validSupabaseProductImageKey(objectName string) bool {
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
