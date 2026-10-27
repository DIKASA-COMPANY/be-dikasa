package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func TestLocalTokensValidateIdentityAndRejectInvalidTokens(t *testing.T) {
	secret := strings.Repeat("a", 32)
	provider, err := NewLocalTokens(secret)
	if err != nil {
		t.Fatal(err)
	}
	session, err := provider.Issue(context.Background(), "postgres-user-id")
	if err != nil {
		t.Fatal(err)
	}
	identity, err := provider.VerifyIDToken(context.Background(), session.IDToken)
	if err != nil || identity.UID != "postgres-user-id" || session.ExpiresIn != 3600 || session.RefreshToken != "" {
		t.Fatalf("unexpected local session: %v", err)
	}
	other, _ := NewLocalTokens(strings.Repeat("b", 32))
	if _, err := other.VerifyIDToken(context.Background(), session.IDToken); err == nil {
		t.Fatal("accepted different signing key")
	}
	cases := []jwt.RegisteredClaims{
		{Subject: "postgres-user-id", Issuer: "dikasa-local", Audience: jwt.ClaimStrings{"dikasa-api"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute))},
		{Subject: "postgres-user-id", Issuer: "other", Audience: jwt.ClaimStrings{"dikasa-api"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
		{Subject: "postgres-user-id", Issuer: "dikasa-local", Audience: jwt.ClaimStrings{"other"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
		{Subject: "", Issuer: "dikasa-local", Audience: jwt.ClaimStrings{"dikasa-api"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
		{Subject: "postgres-user-id", Issuer: "dikasa-local", Audience: jwt.ClaimStrings{"dikasa-api"}},
	}
	for _, claims := range cases {
		token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
		if _, err := provider.VerifyIDToken(context.Background(), token); err == nil {
			t.Fatalf("accepted invalid claims: %+v", claims)
		}
	}
	// Otherwise-valid claims ensure only the algorithm restriction rejects HS384.
	validClaims := jwt.RegisteredClaims{Subject: "postgres-user-id", Issuer: "dikasa-local", Audience: jwt.ClaimStrings{"dikasa-api"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS384, validClaims).SignedString([]byte(secret))
	for _, invalid := range []string{"", "garbage", token, session.IDToken + "tampered"} {
		if _, err := provider.VerifyIDToken(context.Background(), invalid); err == nil {
			t.Fatal("accepted malformed or wrong algorithm token")
		}
	}
	if _, err := NewLocalTokens("short"); err == nil {
		t.Fatal("accepted weak signing key")
	}
	if _, err := provider.Issue(context.Background(), ""); err == nil {
		t.Fatal("accepted empty subject")
	}
}
