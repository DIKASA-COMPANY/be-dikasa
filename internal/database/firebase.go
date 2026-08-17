package database

import (
	"context"
	"errors"
	"fmt"

	firebase "firebase.google.com/go/v4"
	firebaseauth "firebase.google.com/go/v4/auth"
)

// OpenFirebase membuat Firebase Admin Auth client untuk generate dan verifikasi token.
func OpenFirebase(ctx context.Context, projectID string) (*firebaseauth.Client, error) {
	if projectID == "" {
		return nil, errors.New("FIREBASE_PROJECT_ID wajib diatur")
	}
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID})
	if err != nil {
		return nil, fmt.Errorf("inisialisasi Firebase: %w", err)
	}
	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("inisialisasi Firebase Auth: %w", err)
	}
	return client, nil
}
