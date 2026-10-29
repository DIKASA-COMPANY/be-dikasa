package server

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	sharederrors "be-dikasa/internal/shared/errors"
	sharedresponse "be-dikasa/internal/shared/response"
	"be-dikasa/internal/shared/supabaseauth"
	"be-dikasa/internal/user"
)

type identityContextKey struct{}

// Identity is the PostgreSQL identity from a verified provider access token.
type Identity struct {
	UserID string
	Claims map[string]any
}

// VerifiedToken is a verified provider token resolved to a local PostgreSQL user.
type VerifiedToken struct {
	UserID string
	Claims map[string]any
}

// TokenVerifier verifies provider tokens and resolves their local account.
type TokenVerifier interface {
	VerifyAccessToken(ctx context.Context, accessToken string) (*VerifiedToken, error)
}

// SupabaseIdentityProvider verifies one Supabase access token with the Auth service.
type SupabaseIdentityProvider interface {
	VerifyAccessToken(ctx context.Context, accessToken string) (supabaseauth.Identity, error)
}

// SupabaseTokenVerifier maps a verified Supabase subject to an active Dikasa account.
type SupabaseTokenVerifier struct {
	provider SupabaseIdentityProvider
	users    user.Repository
}

// NewSupabaseTokenVerifier creates the default-deny Supabase identity resolver.
func NewSupabaseTokenVerifier(provider SupabaseIdentityProvider, users user.Repository) *SupabaseTokenVerifier {
	return &SupabaseTokenVerifier{provider: provider, users: users}
}

// VerifyAccessToken verifies Supabase and binds confirmed OAuth identities by unique email.
func (verifier *SupabaseTokenVerifier) VerifyAccessToken(ctx context.Context, accessToken string) (*VerifiedToken, error) {
	if verifier == nil || verifier.provider == nil || verifier.users == nil {
		return nil, supabaseauth.ErrInvalidToken
	}
	identity, err := verifier.provider.VerifyAccessToken(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	identities, ok := verifier.users.(user.SupabaseIdentityRepository)
	if !ok {
		return nil, supabaseauth.ErrInvalidToken
	}
	account, err := identities.FindBySupabaseUserID(ctx, identity.ID)
	if errors.Is(err, user.ErrNotFound) {
		account, err = verifier.users.FindByID(ctx, identity.ID)
	}
	if errors.Is(err, user.ErrNotFound) && identity.EmailConfirmed && strings.TrimSpace(identity.Email) != "" {
		account, err = verifier.users.FindByIdentifier(ctx, identity.Email)
	}
	if err != nil || account == nil || !account.IsActive {
		return nil, supabaseauth.ErrInvalidToken
	}
	if account.SupabaseUserID != nil && *account.SupabaseUserID != identity.ID {
		return nil, supabaseauth.ErrInvalidToken
	}
	if err = identities.BindSupabaseUserID(ctx, account.ID, identity.ID); err != nil {
		return nil, supabaseauth.ErrInvalidToken
	}
	return &VerifiedToken{UserID: account.ID, Claims: identity.Claims}, nil
}

// PublicPathMatcher menentukan path infrastruktur yang tidak memerlukan token.
type PublicPathMatcher func(path string) bool

// AuthMiddleware protects routes using the configured token verifier.
func AuthMiddleware(verifier TokenVerifier, isPublic PublicPathMatcher) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isPublic != nil && isPublic(c.Request.URL.Path) {
			c.Next()
			return
		}
		token, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			sharedresponse.WriteError(c.Writer, sharederrors.New(http.StatusUnauthorized, "TOKEN_DIPERLUKAN", "Token autentikasi Bearer diperlukan.", nil))
			c.Abort()
			return
		}
		if verifier == nil {
			sharedresponse.WriteError(c.Writer, sharederrors.New(http.StatusUnauthorized, "TOKEN_TIDAK_VALID", "Token autentikasi tidak valid atau sudah kedaluwarsa.", nil))
			c.Abort()
			return
		}
		verified, err := verifier.VerifyAccessToken(c.Request.Context(), token)
		if err != nil || verified == nil || verified.UserID == "" {
			sharedresponse.WriteError(c.Writer, sharederrors.New(http.StatusUnauthorized, "TOKEN_TIDAK_VALID", "Token autentikasi tidak valid atau sudah kedaluwarsa.", nil))
			c.Abort()
			return
		}
		identity := Identity{UserID: verified.UserID, Claims: verified.Claims}
		ctx := context.WithValue(c.Request.Context(), identityContextKey{}, identity)
		ctx = user.WithAuthenticatedUserID(ctx, verified.UserID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// IdentityFromContext mengambil identitas dari request context.
func IdentityFromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(identityContextKey{}).(Identity)
	return identity, ok
}

func bearerToken(authorization string) (string, bool) {
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}
