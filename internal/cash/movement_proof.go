package cash

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

const maxProofBytes int64 = 3 * 1024 * 1024

// ProofObject contains the only cash-proof reference persisted in PostgreSQL.
type ProofObject struct{ URL string }

// MovementProofStore signs direct uploads, adopts verified proofs, and deletes stored objects.
type MovementProofStore interface {
	Prepare(context.Context, string, string, string, string, string, int64) (supabasestorage.SignedUpload, error)
	Adopt(context.Context, string, string, string, string, string) (ProofObject, error)
	Delete(context.Context, string) error
}

// SupabaseMovementProofStore stores cash proofs through the shared Supabase uploader.
type SupabaseMovementProofStore struct{ uploader *supabasestorage.Uploader }

// NewSupabaseMovementProofStore creates proof storage using the shared upload mechanism.
func NewSupabaseMovementProofStore(uploader *supabasestorage.Uploader) *SupabaseMovementProofStore {
	return &SupabaseMovementProofStore{uploader: uploader}
}

// Prepare validates upload hints and signs one request-bound cash-proof object.
func (store *SupabaseMovementProofStore) Prepare(ctx context.Context, outletID, sessionID, actorID, requestID, contentType string, size int64) (supabasestorage.SignedUpload, error) {
	outlet, err := canonicalProofUUID(outletID)
	if err != nil {
		return supabasestorage.SignedUpload{}, ErrProofInvalid
	}
	session, err := canonicalProofUUID(sessionID)
	if err != nil {
		return supabasestorage.SignedUpload{}, ErrProofInvalid
	}
	actor, err := canonicalProofUUID(actorID)
	if err != nil {
		return supabasestorage.SignedUpload{}, ErrProofInvalid
	}
	requestID, err = canonicalProofUUID(requestID)
	if err != nil {
		return supabasestorage.SignedUpload{}, ErrProofInvalid
	}
	extension, err := imagefile.ValidateIntent(contentType, size, imagefile.StandardPolicy)
	if errors.Is(err, imagefile.ErrTooLarge) {
		return supabasestorage.SignedUpload{}, ErrProofTooLarge
	}
	if err != nil {
		return supabasestorage.SignedUpload{}, ErrProofInvalid
	}
	objectName := fmt.Sprintf("outlets/%s/cash-sessions/%s/users/%s/requests/%s/proofs/%s%s", outlet, session, actor, requestID, uuid.NewString(), extension)
	intent, err := store.uploader.CreateSignedUpload(ctx, objectName)
	if err != nil {
		return supabasestorage.SignedUpload{}, ErrProofStorage
	}
	return intent, nil
}

// Adopt validates a directly uploaded proof before returning its stable public URL.
func (store *SupabaseMovementProofStore) Adopt(ctx context.Context, outletID, sessionID, actorID, requestID, objectName string) (ProofObject, error) {
	outlet, err := canonicalProofUUID(outletID)
	if err != nil {
		return ProofObject{}, ErrProofInvalid
	}
	session, err := canonicalProofUUID(sessionID)
	if err != nil {
		return ProofObject{}, ErrProofInvalid
	}
	actor, err := canonicalProofUUID(actorID)
	if err != nil {
		return ProofObject{}, ErrProofInvalid
	}
	requestID, err = canonicalProofUUID(requestID)
	if err != nil || !validPreparedProofKey(objectName, outlet, session, actor, requestID) {
		return ProofObject{}, ErrProofInvalid
	}
	object, err := store.uploader.Download(ctx, objectName, maxProofBytes)
	if errors.Is(err, supabasestorage.ErrObjectTooLarge) {
		store.deletePrepared(ctx, objectName)
		return ProofObject{}, ErrProofTooLarge
	}
	if errors.Is(err, supabasestorage.ErrObjectNotFound) {
		return ProofObject{}, ErrProofInvalid
	}
	if err != nil {
		return ProofObject{}, ErrProofStorage
	}
	defer object.Body.Close()
	_, err = imagefile.Validate(imagefile.Upload{Reader: object.Body, Size: object.Size, ContentType: object.ContentType, Filename: path.Base(objectName)}, imagefile.StandardPolicy)
	if errors.Is(err, imagefile.ErrTooLarge) {
		store.deletePrepared(ctx, objectName)
		return ProofObject{}, ErrProofTooLarge
	}
	if err != nil {
		store.deletePrepared(ctx, objectName)
		return ProofObject{}, ErrProofInvalid
	}
	publicURL, err := store.uploader.PublicURL(objectName)
	if err != nil {
		return ProofObject{}, ErrProofStorage
	}
	return ProofObject{URL: publicURL}, nil
}

func (store *SupabaseMovementProofStore) deletePrepared(ctx context.Context, objectName string) {
	publicURL, err := store.uploader.PublicURL(objectName)
	if err == nil {
		_ = store.uploader.DeleteURL(ctx, publicURL)
	}
}

// Delete removes a cash proof identified by its persisted Supabase public URL.
func (store *SupabaseMovementProofStore) Delete(ctx context.Context, downloadURL string) error {
	objectName, err := store.uploader.ObjectName(downloadURL)
	if err != nil || (!validSupabaseProofKey(objectName) && !validAnyPreparedProofKey(objectName)) {
		return ErrProofInvalid
	}
	if err = store.uploader.DeleteURL(ctx, downloadURL); err != nil {
		return ErrProofStorage
	}
	return nil
}

func validPreparedProofKey(objectName, outletID, sessionID, actorID, requestID string) bool {
	parts := strings.Split(objectName, "/")
	return len(parts) == 10 && parts[0] == "outlets" && parts[1] == outletID && parts[2] == "cash-sessions" && parts[3] == sessionID && parts[4] == "users" && parts[5] == actorID && parts[6] == "requests" && parts[7] == requestID && parts[8] == "proofs" && validProofFile(parts[9])
}

func validAnyPreparedProofKey(objectName string) bool {
	parts := strings.Split(objectName, "/")
	if len(parts) != 10 || parts[0] != "outlets" || parts[2] != "cash-sessions" || parts[4] != "users" || parts[6] != "requests" || parts[8] != "proofs" {
		return false
	}
	for _, index := range []int{1, 3, 5, 7} {
		if _, err := canonicalProofUUID(parts[index]); err != nil {
			return false
		}
	}
	return validProofFile(parts[9])
}

func validProofFile(filename string) bool {
	extension := path.Ext(filename)
	if extension != ".png" && extension != ".jpg" {
		return false
	}
	_, err := canonicalProofUUID(strings.TrimSuffix(filename, extension))
	return err == nil
}

func canonicalProofUUID(value string) (string, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil || id.String() != value {
		return "", ErrProofInvalid
	}
	return id.String(), nil
}

// validSupabaseProofKey restricts deletion to server-generated cash proof objects.
func validSupabaseProofKey(objectName string) bool {
	parts := strings.Split(objectName, "/")
	if len(parts) != 6 || parts[0] != "outlets" || parts[2] != "cash-sessions" || parts[4] != "proofs" {
		return false
	}
	if _, err := canonicalProofUUID(parts[1]); err != nil {
		return false
	}
	if _, err := canonicalProofUUID(parts[3]); err != nil {
		return false
	}
	extension := path.Ext(parts[5])
	if extension != ".png" && extension != ".jpg" {
		return false
	}
	_, err := canonicalProofUUID(strings.TrimSuffix(parts[5], extension))
	return err == nil
}
