// Package supabaseauth provides the small Supabase Auth REST surface required by the backend.
package supabaseauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	// ErrInvalidCredentials indicates that Supabase rejected an email/password sign-in.
	ErrInvalidCredentials = errors.New("invalid Supabase credentials")
	// ErrInvalidToken indicates that Supabase rejected an access token.
	ErrInvalidToken = errors.New("invalid Supabase access token")
	// ErrUserExists indicates that an Auth user already occupies the requested identity.
	ErrUserExists = errors.New("Supabase Auth user already exists")
	// ErrUserNotFound indicates that no Supabase Auth user matches an exact email.
	ErrUserNotFound = errors.New("Supabase Auth user not found")
	// ErrUnavailable indicates an invalid response or transport failure from Supabase Auth.
	ErrUnavailable = errors.New("Supabase Auth unavailable")
)

// Session is a Supabase Auth password session.
type Session struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    int
	UserID       string
}

// Identity is the verified Supabase Auth user associated with an access token.
type Identity struct {
	ID             string
	Email          string
	EmailConfirmed bool
	Claims         map[string]any
}

// UserImport contains an existing Dikasa user's stable UUID and bcrypt password hash.
type UserImport struct {
	ID             string
	Email          string
	PasswordHash   string
	EmailConfirmed bool
}

// Option customizes Client dependencies.
type Option func(*Client)

// WithHTTPClient replaces the bounded default HTTP client, primarily for tests.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		if httpClient != nil {
			client.httpClient = httpClient
		}
	}
}

// Client calls Supabase Auth with separate publishable and server-only credentials.
type Client struct {
	projectURL     *url.URL
	publishableKey string
	secretKey      string
	httpClient     *http.Client
}

// NewClient validates Supabase configuration and creates an Auth REST client.
func NewClient(projectURL, publishableKey, secretKey string, options ...Option) (*Client, error) {
	parsed, err := url.Parse(strings.TrimSpace(projectURL))
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("SUPABASE_URL tidak valid")
	}
	if strings.TrimSpace(publishableKey) == "" {
		return nil, fmt.Errorf("SUPABASE_PUBLISHABLE_KEY wajib diatur")
	}
	if strings.TrimSpace(secretKey) == "" {
		return nil, fmt.Errorf("SUPABASE_SECRET_KEY wajib diatur")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	client := &Client{
		projectURL: parsed, publishableKey: strings.TrimSpace(publishableKey), secretKey: strings.TrimSpace(secretKey),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	for _, option := range options {
		option(client)
	}
	return client, nil
}

// SignIn exchanges an email and password for a Supabase user session.
func (client *Client) SignIn(ctx context.Context, email, password string) (Session, error) {
	payload := struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}{Email: strings.ToLower(strings.TrimSpace(email)), Password: password}
	response, err := client.request(ctx, http.MethodPost, "/auth/v1/token?grant_type=password", payload, client.publishableKey, "")
	if err != nil {
		return Session{}, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusBadRequest || response.StatusCode == http.StatusUnauthorized {
		return Session{}, ErrInvalidCredentials
	}
	if response.StatusCode != http.StatusOK {
		return Session{}, fmt.Errorf("%w: sign-in status %d", ErrUnavailable, response.StatusCode)
	}
	var body struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		User         struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err = decodeJSON(response.Body, &body); err != nil || body.AccessToken == "" || body.RefreshToken == "" || body.ExpiresIn < 1 || body.User.ID == "" {
		return Session{}, fmt.Errorf("%w: invalid sign-in response", ErrUnavailable)
	}
	return Session{AccessToken: body.AccessToken, RefreshToken: body.RefreshToken, TokenType: body.TokenType, ExpiresIn: body.ExpiresIn, UserID: body.User.ID}, nil
}

// ImportUser creates a confirmed Supabase Auth user while preserving its UUID and bcrypt hash.
func (client *Client) ImportUser(ctx context.Context, value UserImport) error {
	payload := struct {
		ID             string         `json:"id"`
		Email          string         `json:"email"`
		PasswordHash   string         `json:"password_hash"`
		EmailConfirmed bool           `json:"email_confirm"`
		AppMetadata    map[string]any `json:"app_metadata"`
	}{
		ID: value.ID, Email: strings.ToLower(strings.TrimSpace(value.Email)), PasswordHash: value.PasswordHash,
		EmailConfirmed: value.EmailConfirmed, AppMetadata: map[string]any{"dikasa_user_id": value.ID},
	}
	response, err := client.request(ctx, http.MethodPost, "/auth/v1/admin/users", payload, client.secretKey, client.secretKey)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnprocessableEntity || response.StatusCode == http.StatusConflict {
		return ErrUserExists
	}
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		return fmt.Errorf("%w: import user status %d", ErrUnavailable, response.StatusCode)
	}
	return nil
}

// UpdateUserCredentials synchronizes an existing Supabase Auth user's email and bcrypt hash.
func (client *Client) UpdateUserCredentials(ctx context.Context, userID, email, passwordHash string) error {
	payload := struct {
		Email          string `json:"email"`
		PasswordHash   string `json:"password_hash"`
		EmailConfirmed bool   `json:"email_confirm"`
	}{Email: strings.ToLower(strings.TrimSpace(email)), PasswordHash: passwordHash, EmailConfirmed: true}
	endpoint := "/auth/v1/admin/users/" + url.PathEscape(strings.TrimSpace(userID))
	response, err := client.request(ctx, http.MethodPut, endpoint, payload, client.secretKey, client.secretKey)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: update user status %d", ErrUnavailable, response.StatusCode)
	}
	return nil
}

// FindUserByEmail finds an exact Auth email match for one-time OAuth/password linking.
func (client *Client) FindUserByEmail(ctx context.Context, email string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	const perPage = 1000
	for page := 1; ; page++ {
		endpoint := fmt.Sprintf("/auth/v1/admin/users?filter=%s&page=%d&per_page=%d", url.QueryEscape(normalized), page, perPage)
		response, err := client.request(ctx, http.MethodGet, endpoint, nil, client.secretKey, client.secretKey)
		if err != nil {
			return "", err
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			return "", fmt.Errorf("%w: list users status %d", ErrUnavailable, response.StatusCode)
		}
		var body struct {
			Users []struct {
				ID    string `json:"id"`
				Email string `json:"email"`
			} `json:"users"`
		}
		decodeErr := decodeJSON(response.Body, &body)
		response.Body.Close()
		if decodeErr != nil {
			return "", fmt.Errorf("%w: invalid list users response", ErrUnavailable)
		}
		for _, candidate := range body.Users {
			if strings.EqualFold(strings.TrimSpace(candidate.Email), normalized) && candidate.ID != "" {
				return candidate.ID, nil
			}
		}
		if len(body.Users) < perPage {
			return "", ErrUserNotFound
		}
	}
}

// VerifyAccessToken asks Supabase Auth to verify a session and returns its canonical user.
func (client *Client) VerifyAccessToken(ctx context.Context, accessToken string) (Identity, error) {
	response, err := client.request(ctx, http.MethodGet, "/auth/v1/user", nil, client.publishableKey, accessToken)
	if err != nil {
		return Identity{}, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return Identity{}, ErrInvalidToken
	}
	if response.StatusCode != http.StatusOK {
		return Identity{}, fmt.Errorf("%w: verify token status %d", ErrUnavailable, response.StatusCode)
	}
	var body struct {
		ID               string         `json:"id"`
		Email            string         `json:"email"`
		EmailConfirmedAt *time.Time     `json:"email_confirmed_at"`
		Role             string         `json:"role"`
		AppMetadata      map[string]any `json:"app_metadata"`
		UserMetadata     map[string]any `json:"user_metadata"`
	}
	if err = decodeJSON(response.Body, &body); err != nil || body.ID == "" {
		return Identity{}, fmt.Errorf("%w: invalid user response", ErrUnavailable)
	}
	claims := map[string]any{"sub": body.ID, "email": body.Email, "role": body.Role}
	for key, value := range body.AppMetadata {
		claims[key] = value
	}
	claims["app_metadata"] = body.AppMetadata
	claims["user_metadata"] = body.UserMetadata
	return Identity{ID: body.ID, Email: body.Email, EmailConfirmed: body.EmailConfirmedAt != nil, Claims: claims}, nil
}

func (client *Client) request(ctx context.Context, method, endpoint string, payload any, apiKey, bearer string) (*http.Response, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("%w: encode request", ErrUnavailable)
		}
		body = bytes.NewReader(encoded)
	}
	target := strings.TrimRight(client.projectURL.String(), "/") + endpoint
	request, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, fmt.Errorf("%w: create request", ErrUnavailable)
	}
	request.Header.Set("apikey", apiKey)
	request.Header.Set("Accept", "application/json")
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return response, nil
}

func decodeJSON(reader io.Reader, destination any) error {
	return json.NewDecoder(io.LimitReader(reader, 1<<20)).Decode(destination)
}
