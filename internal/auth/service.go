package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"

	"be-dikasa/internal/user"
)

const dummyPasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

const defaultTokenExchangeURL = "https://identitytoolkit.googleapis.com/v1/accounts:signInWithCustomToken"

var (
	// ErrInvalidCredentials menandakan identifier atau password tidak valid.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrUserDisabled menandakan user PostgreSQL dinonaktifkan.
	ErrUserDisabled = errors.New("user disabled")
	// ErrProviderUnavailable menandakan Firebase gagal menerbitkan token.
	ErrProviderUnavailable = errors.New("provider unavailable")
)

// SignInProvider mendefinisikan use case login untuk handler Huma.
type SignInProvider interface {
	SignIn(ctx context.Context, identifier, password string) (Session, error)
}

// TokenIssuer menerbitkan Firebase ID token untuk UUID user PostgreSQL.
type TokenIssuer interface {
	Issue(ctx context.Context, uid string) (Session, error)
}

// Service mengautentikasi kredensial PostgreSQL sebelum meminta token Firebase.
type Service struct {
	users  user.Repository
	issuer TokenIssuer
}

// NewService membuat auth service dengan repository user dan penerbit token.
func NewService(users user.Repository, issuer TokenIssuer) *Service {
	return &Service{users: users, issuer: issuer}
}

// SignIn memvalidasi identifier dan password hash PostgreSQL lalu menerbitkan token Firebase.
func (service *Service) SignIn(ctx context.Context, identifier, password string) (Session, error) {
	account, err := service.users.FindByIdentifier(ctx, identifier)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			// Tetap jalankan bcrypt untuk mengurangi perbedaan waktu antara user ada dan tidak ada.
			_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(password))
			return Session{}, ErrInvalidCredentials
		}
		return Session{}, err
	}
	if !account.IsActive {
		return Session{}, ErrUserDisabled
	}
	if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(password)); err != nil {
		return Session{}, ErrInvalidCredentials
	}

	session, err := service.issuer.Issue(ctx, account.ID)
	if err != nil {
		return Session{}, ErrProviderUnavailable
	}
	return session, nil
}

// CustomTokenClient membuat Firebase custom token untuk UID lokal.
type CustomTokenClient interface {
	CustomToken(ctx context.Context, uid string) (string, error)
}

// FirebaseTokenIssuerOption mengubah konfigurasi FirebaseTokenIssuer.
type FirebaseTokenIssuerOption func(*FirebaseTokenIssuer)

// FirebaseTokenIssuer membuat custom token lalu menukarnya menjadi ID dan refresh token.
type FirebaseTokenIssuer struct {
	client      CustomTokenClient
	webAPIKey   string
	exchangeURL string
	httpClient  *http.Client
}

// NewFirebaseTokenIssuer membuat penerbit token Firebase tanpa menyimpan data bisnis di Firebase.
func NewFirebaseTokenIssuer(client CustomTokenClient, webAPIKey string, options ...FirebaseTokenIssuerOption) *FirebaseTokenIssuer {
	issuer := &FirebaseTokenIssuer{
		client: client, webAPIKey: webAPIKey, exchangeURL: defaultTokenExchangeURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	for _, option := range options {
		option(issuer)
	}
	return issuer
}

// WithTokenExchangeURL mengganti endpoint pertukaran token untuk pengujian.
func WithTokenExchangeURL(exchangeURL string) FirebaseTokenIssuerOption {
	return func(issuer *FirebaseTokenIssuer) {
		issuer.exchangeURL = exchangeURL
	}
}

// Issue menerbitkan Firebase ID token dengan UID yang sama dengan UUID user PostgreSQL.
func (issuer *FirebaseTokenIssuer) Issue(ctx context.Context, uid string) (Session, error) {
	customToken, err := issuer.client.CustomToken(ctx, uid)
	if err != nil {
		return Session{}, ErrProviderUnavailable
	}

	payload, err := json.Marshal(struct {
		Token             string `json:"token"`
		ReturnSecureToken bool   `json:"returnSecureToken"`
	}{Token: customToken, ReturnSecureToken: true})
	if err != nil {
		return Session{}, ErrProviderUnavailable
	}
	endpoint, err := url.Parse(issuer.exchangeURL)
	if err != nil {
		return Session{}, ErrProviderUnavailable
	}
	query := endpoint.Query()
	query.Set("key", issuer.webAPIKey)
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return Session{}, ErrProviderUnavailable
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := issuer.httpClient.Do(request)
	if err != nil {
		return Session{}, ErrProviderUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Session{}, ErrProviderUnavailable
	}

	var tokenResponse struct {
		IDToken      string `json:"idToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresIn    string `json:"expiresIn"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&tokenResponse); err != nil {
		return Session{}, ErrProviderUnavailable
	}
	expiresIn, err := strconv.Atoi(tokenResponse.ExpiresIn)
	if err != nil || tokenResponse.IDToken == "" || tokenResponse.RefreshToken == "" {
		return Session{}, ErrProviderUnavailable
	}
	return Session{
		IDToken: tokenResponse.IDToken, RefreshToken: tokenResponse.RefreshToken,
		ExpiresIn: expiresIn, FirebaseUID: uid,
	}, nil
}
