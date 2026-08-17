package user

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

// GORMRepository menyimpan dan mengambil user melalui PostgreSQL/GORM.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository membuat repository user berbasis GORM.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// FindByIdentifier mencari user berdasarkan username, nomor HP, atau email.
func (repository *GORMRepository) FindByIdentifier(ctx context.Context, identifier string) (*User, error) {
	identifier = strings.TrimSpace(identifier)
	var account User
	err := repository.db.WithContext(ctx).
		Where("LOWER(username) = LOWER(?) OR phone_number = ? OR LOWER(email) = LOWER(?)", identifier, identifier, identifier).
		First(&account).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &account, nil
}

// FindByID mencari user berdasarkan UUID PostgreSQL.
func (repository *GORMRepository) FindByID(ctx context.Context, id string) (*User, error) {
	var account User
	err := repository.db.WithContext(ctx).First(&account, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &account, nil
}
