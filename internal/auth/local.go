package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	firebaseauth "firebase.google.com/go/v4/auth"
	"github.com/golang-jwt/jwt/v4"
)

// LocalTokens issues and verifies one-hour HMAC sessions for local development.
// It implements the same boundaries as the Firebase issuer and verifier.
type LocalTokens struct{ secret []byte }

// NewLocalTokens requires an explicit signing secret of at least 32 bytes.
func NewLocalTokens(secret string) (*LocalTokens, error) {
	if len(strings.TrimSpace(secret)) < 32 {
		return nil, errors.New("LOCAL_AUTH_SECRET must contain at least 32 bytes")
	}
	return &LocalTokens{secret: []byte(secret)}, nil
}

// Issue creates an access token bound to the PostgreSQL user UUID.
// Local sessions have no refresh token; clients log in again after expiry.
func (provider *LocalTokens) Issue(_ context.Context, uid string) (Session, error) {
	if strings.TrimSpace(uid) == "" {
		return Session{}, errors.New("user ID is required")
	}
	now := time.Now().UTC()
	claims := jwt.RegisteredClaims{
		Subject: uid, Issuer: "dikasa-local", Audience: jwt.ClaimStrings{"dikasa-api"},
		IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(provider.secret)
	if err != nil {
		return Session{}, err
	}
	return Session{IDToken: token, ExpiresIn: 3600, FirebaseUID: uid}, nil
}

// VerifyIDToken rejects expired, tampered, foreign-audience and non-HS256 tokens.
// The Firebase token type is retained as an adapter for the existing middleware.
func (provider *LocalTokens) VerifyIDToken(_ context.Context, value string) (*firebaseauth.Token, error) {
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(value, claims, func(*jwt.Token) (any, error) {
		return provider.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, err
	}
	if !token.Valid || claims.Subject == "" || claims.ExpiresAt == nil ||
		!claims.VerifyIssuer("dikasa-local", true) || !claims.VerifyAudience("dikasa-api", true) {
		return nil, errors.New("invalid local session")
	}
	return &firebaseauth.Token{UID: claims.Subject, Claims: map[string]any{"sub": claims.Subject}}, nil
}
