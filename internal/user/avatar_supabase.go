package user

import (
	"be-dikasa/internal/shared/supabasestorage"
	"bytes"
	"context"
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

// Save validates an avatar, uploads it to Supabase, and returns only its public URL.
func (store *SupabaseAvatarStore) Save(ctx context.Context, outletID, userID string, upload *AvatarUpload) (*AvatarImage, error) {
	outlet, err := uuid.Parse(outletID)
	if err != nil || outlet == uuid.Nil || outlet.String() != outletID {
		return nil, ErrInvalidAvatar
	}
	account, err := uuid.Parse(userID)
	if err != nil || account == uuid.Nil || account.String() != userID {
		return nil, ErrInvalidAvatar
	}
	content, extension, declared, err := validateAvatar(upload)
	if err != nil {
		return nil, err
	}
	objectName := fmt.Sprintf("outlets/%s/users/%s/avatar/%s%s", outlet, account, uuid.NewString(), extension)
	downloadURL, err := store.uploader.Upload(ctx, objectName, bytes.NewReader(content), declared)
	if err != nil {
		return nil, fmt.Errorf("%w: upload", ErrAvatarStorage)
	}
	return &AvatarImage{URL: downloadURL}, nil
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
