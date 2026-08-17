package server

import (
	"context"
	"net/http"
	"strings"

	firebaseauth "firebase.google.com/go/v4/auth"
	"github.com/gin-gonic/gin"

	sharederrors "be-dikasa/internal/shared/errors"
	sharedresponse "be-dikasa/internal/shared/response"
	"be-dikasa/internal/user"
)

type identityContextKey struct{}

// Identity adalah identitas user PostgreSQL dari Firebase ID token terverifikasi.
type Identity struct {
	UserID string
	Claims map[string]any
}

// TokenVerifier memverifikasi Firebase ID token.
type TokenVerifier interface {
	VerifyIDToken(ctx context.Context, idToken string) (*firebaseauth.Token, error)
}

// PublicPathMatcher menentukan path infrastruktur yang tidak memerlukan token.
type PublicPathMatcher func(path string) bool

// AuthMiddleware melindungi route Gin menggunakan Firebase ID token.
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
		firebaseToken, err := verifier.VerifyIDToken(c.Request.Context(), token)
		if err != nil || firebaseToken == nil || firebaseToken.UID == "" {
			sharedresponse.WriteError(c.Writer, sharederrors.New(http.StatusUnauthorized, "TOKEN_TIDAK_VALID", "Token autentikasi tidak valid atau sudah kedaluwarsa.", nil))
			c.Abort()
			return
		}
		identity := Identity{UserID: firebaseToken.UID, Claims: firebaseToken.Claims}
		ctx := context.WithValue(c.Request.Context(), identityContextKey{}, identity)
		ctx = user.WithAuthenticatedUserID(ctx, firebaseToken.UID)
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
