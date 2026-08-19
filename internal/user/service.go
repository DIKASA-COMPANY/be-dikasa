package user

import "context"

type authenticatedUserIDKey struct{}

// Service menyediakan use case user tanpa bergantung pada transport HTTP.
type Service struct {
	repository Repository
	settings   SettingsRepository
	hasher     CredentialHasher
	avatars    AvatarStore
}

// NewService membuat user service dengan repository yang diberikan.
func NewService(repository Repository, options ...ServiceOption) *Service {
	service := &Service{repository: repository, hasher: bcryptCredentialHasher{}, avatars: NewDisabledAvatarStore()}
	service.settings, _ = repository.(SettingsRepository)
	for _, option := range options {
		option(service)
	}
	return service
}

// FindByIdentifier mencari user menggunakan username, nomor HP, atau email.
func (service *Service) FindByIdentifier(ctx context.Context, identifier string) (*User, error) {
	return service.repository.FindByIdentifier(ctx, identifier)
}

// FindByID mencari user menggunakan UUID PostgreSQL.
func (service *Service) FindByID(ctx context.Context, id string) (*User, error) {
	return service.repository.FindByID(ctx, id)
}

// WithAuthenticatedUserID menyimpan UUID user terverifikasi pada context.
func WithAuthenticatedUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, authenticatedUserIDKey{}, userID)
}

// AuthenticatedUserIDFromContext mengambil UUID user terverifikasi dari context.
func AuthenticatedUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(authenticatedUserIDKey{}).(string)
	return userID, ok && userID != ""
}
