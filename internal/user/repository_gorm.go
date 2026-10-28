package user

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// GORMRepository menyimpan dan mengambil user melalui PostgreSQL/GORM.
type GORMRepository struct {
	db *gorm.DB
}

// UpdateProfile memperbarui hanya data profil yang dapat dikelola pemilik akun.
func (repository *GORMRepository) UpdateProfile(ctx context.Context, account *User) error {
	result := repository.db.WithContext(ctx).Model(&User{}).Where("id = ?", account.ID).Updates(map[string]any{
		"first_name": account.FirstName, "last_name": account.LastName, "phone_number": account.PhoneNumber,
		"email": account.Email, "avatar_url": account.AvatarURL,
	})
	if result.Error != nil {
		return normalizeUserWriteError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdatePINHash mengganti hash PIN tanpa menyimpan nilai plaintext.
func (repository *GORMRepository) UpdatePINHash(ctx context.Context, userID, hash string) error {
	result := repository.db.WithContext(ctx).Model(&User{}).Where("id = ?", userID).Update("pin_hash", hash)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdatePasswordHash mengganti hash password tanpa menyimpan nilai plaintext.
func (repository *GORMRepository) UpdatePasswordHash(ctx context.Context, userID, hash string) error {
	result := repository.db.WithContext(ctx).Model(&User{}).Where("id = ?", userID).Update("password_hash", hash)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func normalizeUserWriteError(err error) error {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrConflict
	}
	if errors.Is(err, gorm.ErrForeignKeyViolated) {
		return ErrInvalidProfile
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23503" {
		return ErrInvalidProfile
	}
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return ErrConflict
	}
	return err
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
	err := repository.db.WithContext(ctx).Preload("Role").Preload("Outlet").Preload("Gender").First(&account, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &account, nil
}

// FindByIDWithRole mencari user sekaligus memuat role untuk keputusan otorisasi.
func (repository *GORMRepository) FindByIDWithRole(ctx context.Context, id string) (*User, error) {
	var account User
	err := repository.db.WithContext(ctx).Preload("Role").First(&account, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &account, nil
}
