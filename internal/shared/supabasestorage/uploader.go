// Package supabasestorage provides the single Supabase Storage mechanism used by image APIs.
package supabasestorage

import (
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
	// ErrInvalidURL indicates a URL outside the configured public Supabase bucket.
	ErrInvalidURL = errors.New("invalid Supabase Storage public URL")
	// ErrStorage indicates a Supabase Storage transport or API failure.
	ErrStorage = errors.New("Supabase Storage operation failed")
)

// Option customizes Uploader dependencies.
type Option func(*Uploader)

// WithHTTPClient replaces the bounded default HTTP client, primarily for tests.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(uploader *Uploader) {
		if httpClient != nil {
			uploader.httpClient = httpClient
		}
	}
}

// Uploader uploads validated images to one public Supabase Storage bucket.
type Uploader struct {
	projectURL *url.URL
	bucket     string
	secretKey  string
	httpClient *http.Client
}

// NewUploader validates configuration and creates a Supabase Storage client.
func NewUploader(projectURL, bucket, secretKey string, options ...Option) (*Uploader, error) {
	parsed, err := url.Parse(strings.TrimSpace(projectURL))
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("SUPABASE_URL tidak valid")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	bucket = strings.TrimSpace(bucket)
	if bucket == "" || strings.Contains(bucket, "/") || strings.Contains(bucket, "..") {
		return nil, fmt.Errorf("SUPABASE_STORAGE_BUCKET tidak valid")
	}
	if strings.TrimSpace(secretKey) == "" {
		return nil, fmt.Errorf("SUPABASE_SECRET_KEY wajib diatur")
	}
	uploader := &Uploader{projectURL: parsed, bucket: bucket, secretKey: strings.TrimSpace(secretKey), httpClient: &http.Client{Timeout: 15 * time.Second}}
	for _, option := range options {
		option(uploader)
	}
	return uploader, nil
}

// VerifyBucket confirms that the configured bucket exists and is public.
func (uploader *Uploader) VerifyBucket(ctx context.Context) error {
	request, err := uploader.newRequest(ctx, http.MethodGet, "/storage/v1/bucket/"+url.PathEscape(uploader.bucket), nil, "")
	if err != nil {
		return err
	}
	response, err := uploader.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("%w: verify bucket: %v", ErrStorage, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: verify bucket status %d", ErrStorage, response.StatusCode)
	}
	var body struct {
		ID     string `json:"id"`
		Public bool   `json:"public"`
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&body); err != nil || body.ID != uploader.bucket || !body.Public {
		return fmt.Errorf("%w: bucket harus ada dan public", ErrStorage)
	}
	return nil
}

// Upload stores an object without overwriting and returns its stable public URL.
func (uploader *Uploader) Upload(ctx context.Context, objectName string, source io.Reader, contentType string) (string, error) {
	escapedObject, err := escapedObjectName(objectName)
	if err != nil {
		return "", err
	}
	endpoint := "/storage/v1/object/" + url.PathEscape(uploader.bucket) + "/" + escapedObject
	request, err := uploader.newRequest(ctx, http.MethodPost, endpoint, source, contentType)
	if err != nil {
		return "", err
	}
	request.Header.Set("x-upsert", "false")
	response, err := uploader.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("%w: upload object: %v", ErrStorage, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("%w: upload status %d", ErrStorage, response.StatusCode)
	}
	return strings.TrimRight(uploader.projectURL.String(), "/") + "/storage/v1/object/public/" + url.PathEscape(uploader.bucket) + "/" + escapedObject, nil
}

// ObjectName extracts an object name only from this uploader's public bucket URL.
func (uploader *Uploader) ObjectName(publicURL string) (string, error) {
	parsed, err := url.Parse(publicURL)
	if err != nil || parsed.Scheme != uploader.projectURL.Scheme || parsed.Host != uploader.projectURL.Host || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", ErrInvalidURL
	}
	prefix := strings.TrimRight(uploader.projectURL.EscapedPath(), "/") + "/storage/v1/object/public/" + url.PathEscape(uploader.bucket) + "/"
	if !strings.HasPrefix(parsed.EscapedPath(), prefix) {
		return "", ErrInvalidURL
	}
	escaped := strings.TrimPrefix(parsed.EscapedPath(), prefix)
	name, err := url.PathUnescape(escaped)
	if err != nil || name == "" || strings.ContainsRune(name, '\x00') {
		return "", ErrInvalidURL
	}
	if _, err = escapedObjectName(name); err != nil {
		return "", ErrInvalidURL
	}
	return name, nil
}

// DeleteURL removes the trusted object addressed by a configured public bucket URL.
func (uploader *Uploader) DeleteURL(ctx context.Context, publicURL string) error {
	objectName, err := uploader.ObjectName(publicURL)
	if err != nil {
		return err
	}
	escapedObject, _ := escapedObjectName(objectName)
	endpoint := "/storage/v1/object/" + url.PathEscape(uploader.bucket) + "/" + escapedObject
	request, err := uploader.newRequest(ctx, http.MethodDelete, endpoint, nil, "")
	if err != nil {
		return err
	}
	response, err := uploader.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("%w: delete object: %v", ErrStorage, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("%w: delete status %d", ErrStorage, response.StatusCode)
	}
	return nil
}

func (uploader *Uploader) newRequest(ctx context.Context, method, endpoint string, body io.Reader, contentType string) (*http.Request, error) {
	target := strings.TrimRight(uploader.projectURL.String(), "/") + endpoint
	request, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, fmt.Errorf("%w: create request", ErrStorage)
	}
	request.Header.Set("apikey", uploader.secretKey)
	request.Header.Set("Authorization", "Bearer "+uploader.secretKey)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	return request, nil
}

func escapedObjectName(objectName string) (string, error) {
	parts := strings.Split(objectName, "/")
	if len(parts) < 2 {
		return "", ErrInvalidURL
	}
	escaped := make([]string, len(parts))
	for index, part := range parts {
		if part == "" || part == "." || part == ".." || strings.ContainsRune(part, '\x00') {
			return "", ErrInvalidURL
		}
		escaped[index] = url.PathEscape(part)
	}
	return strings.Join(escaped, "/"), nil
}
