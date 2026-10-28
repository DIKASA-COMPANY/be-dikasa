package cash

import (
	"be-dikasa/internal/shared/firebaseimage"
	"be-dikasa/internal/shared/imagefile"
	"bytes"
	"context"
	"path"
	"strings"

	"github.com/google/uuid"
)

const maxProofBytes int64 = 3 * 1024 * 1024

// ProofObject contains the only cash-proof reference persisted in PostgreSQL.
type ProofObject struct{ URL string }

// MovementProofStore uploads and deletes cash proof images through Firebase URLs.
type MovementProofStore interface {
	Save(context.Context, string, string, imagefile.Image) (ProofObject, error)
	Delete(context.Context, string) error
}

// FirebaseMovementProofStore stores cash proofs through the shared Firebase uploader.
type FirebaseMovementProofStore struct{ uploader *firebaseimage.Uploader }

// NewFirebaseMovementProofStore creates proof storage using the shared upload mechanism.
func NewFirebaseMovementProofStore(uploader *firebaseimage.Uploader) *FirebaseMovementProofStore {
	return &FirebaseMovementProofStore{uploader: uploader}
}

// Save writes validated evidence and returns only its Firebase download URL.
func (store *FirebaseMovementProofStore) Save(ctx context.Context, outletID, sessionID string, value imagefile.Image) (ProofObject, error) {
	outlet, err := canonicalProofUUID(outletID)
	if err != nil {
		return ProofObject{}, ErrProofInvalid
	}
	session, err := canonicalProofUUID(sessionID)
	if err != nil || len(value.Bytes) < 1 || int64(len(value.Bytes)) > maxProofBytes || (value.Extension != ".png" && value.Extension != ".jpg") {
		return ProofObject{}, ErrProofInvalid
	}
	objectName := "outlets/" + outlet + "/cash-sessions/" + session + "/proofs/" + uuid.NewString() + value.Extension
	downloadURL, err := store.uploader.Upload(ctx, objectName, bytes.NewReader(value.Bytes), value.ContentType)
	if err != nil {
		return ProofObject{}, ErrProofStorage
	}
	return ProofObject{URL: downloadURL}, nil
}

// Delete removes a cash proof identified by its persisted Firebase download URL.
func (store *FirebaseMovementProofStore) Delete(ctx context.Context, downloadURL string) error {
	objectName, err := store.uploader.ObjectName(downloadURL)
	if err != nil || !validFirebaseProofKey(objectName) {
		return ErrProofInvalid
	}
	if err = store.uploader.DeleteURL(ctx, downloadURL); err != nil {
		return ErrProofStorage
	}
	return nil
}

func canonicalProofUUID(value string) (string, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil || id.String() != value {
		return "", ErrProofInvalid
	}
	return id.String(), nil
}

// validFirebaseProofKey restricts deletion to server-generated cash proof objects.
func validFirebaseProofKey(objectName string) bool {
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
