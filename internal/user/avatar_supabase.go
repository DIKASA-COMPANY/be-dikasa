package user

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

const maxAvatarBytes int64 = 3 * 1024 * 1024

// SupabaseAvatarStore stores avatars through the shared Supabase uploader.
type SupabaseAvatarStore struct{ uploader *supabasestorage.Uploader }

// NewSupabaseAvatarStore creates avatar storage using the shared upload mechanism.
func NewSupabaseAvatarStore(uploader *supabasestorage.Uploader) *SupabaseAvatarStore {
	return &SupabaseAvatarStore{uploader: uploader}
}

// Prepare validates upload hints and signs one user-bound avatar object.
func (store *SupabaseAvatarStore) Prepare(ctx context.Context, outletID, userID, contentType string, size int64) (supabasestorage.SignedUpload, error) {
	outlet, err := canonicalAvatarUUID(outletID)
	if err != nil {
		return supabasestorage.SignedUpload{}, ErrInvalidAvatar
	}
	account, err := canonicalAvatarUUID(userID)
	if err != nil {
		return supabasestorage.SignedUpload{}, ErrInvalidAvatar
	}
	extension, err := imagefile.ValidateIntent(contentType, size, imagefile.StandardPolicy)
	if errors.Is(err, imagefile.ErrTooLarge) {
		return supabasestorage.SignedUpload{}, ErrAvatarTooLarge
	}
	if err != nil {
		return supabasestorage.SignedUpload{}, ErrInvalidAvatar
	}
	objectName := fmt.Sprintf("outlets/%s/users/%s/avatar/%s%s", outlet, account, uuid.NewString(), extension)
	intent, err := store.uploader.CreateSignedUpload(ctx, objectName)
	if err != nil {
		return supabasestorage.SignedUpload{}, fmt.Errorf("%w: sign upload", ErrAvatarStorage)
	}
	return intent, nil
}

// Adopt validates a directly uploaded avatar before returning its stable public URL.
func (store *SupabaseAvatarStore) Adopt(ctx context.Context, outletID, userID, objectName string) (*AvatarImage, error) {
	outlet, err := canonicalAvatarUUID(outletID)
	if err != nil {
		return nil, ErrInvalidAvatar
	}
	account, err := canonicalAvatarUUID(userID)
	if err != nil || !validPreparedAvatarKey(objectName, outlet, account) {
		return nil, ErrInvalidAvatar
	}
	object, err := store.uploader.Download(ctx, objectName, maxAvatarBytes)
	if errors.Is(err, supabasestorage.ErrObjectTooLarge) {
		store.deletePrepared(ctx, objectName)
		return nil, ErrAvatarTooLarge
	}
	if errors.Is(err, supabasestorage.ErrObjectNotFound) {
		return nil, ErrInvalidAvatar
	}
	if err != nil {
		return nil, fmt.Errorf("%w: download", ErrAvatarStorage)
	}
	defer object.Body.Close()
	_, err = imagefile.Validate(imagefile.Upload{Reader: object.Body, Size: object.Size, ContentType: object.ContentType, Filename: path.Base(objectName)}, imagefile.StandardPolicy)
	if errors.Is(err, imagefile.ErrTooLarge) {
		store.deletePrepared(ctx, objectName)
		return nil, ErrAvatarTooLarge
	}
	if err != nil {
		store.deletePrepared(ctx, objectName)
		return nil, ErrInvalidAvatar
	}
	publicURL, err := store.uploader.PublicURL(objectName)
	if err != nil {
		return nil, fmt.Errorf("%w: public URL", ErrAvatarStorage)
	}
	return &AvatarImage{URL: publicURL}, nil
}

func (store *SupabaseAvatarStore) deletePrepared(ctx context.Context, objectName string) {
	publicURL, err := store.uploader.PublicURL(objectName)
	if err == nil {
		_ = store.uploader.DeleteURL(ctx, publicURL)
	}
}

// Delete removes an avatar identified by its persisted Supabase public URL.
func (store *SupabaseAvatarStore) Delete(ctx context.Context, downloadURL string) error {
	if strings.TrimSpace(downloadURL) == "" {
		return nil
	}
	objectName, err := store.uploader.ObjectName(downloadURL)
	if err != nil || !validSupabaseAvatarKey(objectName) {
		return ErrInvalidAvatar
	}
	if err = store.uploader.DeleteURL(ctx, downloadURL); err != nil {
		return fmt.Errorf("%w: delete", ErrAvatarStorage)
	}
	return nil
}

// validSupabaseAvatarKey restricts deletion to server-generated avatar objects.
func validSupabaseAvatarKey(objectName string) bool {
	parts := strings.Split(objectName, "/")
	if len(parts) != 6 || parts[0] != "outlets" || parts[2] != "users" || parts[4] != "avatar" {
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

func canonicalAvatarUUID(value string) (string, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil || id.String() != value {
		return "", ErrInvalidAvatar
	}
	return id.String(), nil
}

func validPreparedAvatarKey(objectName, outletID, userID string) bool {
	parts := strings.Split(objectName, "/")
	return len(parts) == 6 && parts[0] == "outlets" && parts[1] == outletID && parts[2] == "users" && parts[3] == userID && parts[4] == "avatar" && validAvatarFile(parts[5])
}

func validAvatarFile(filename string) bool {
	extension := path.Ext(filename)
	if extension != ".png" && extension != ".jpg" {
		return false
	}
	_, err := canonicalAvatarUUID(strings.TrimSuffix(filename, extension))
	return err == nil
}
