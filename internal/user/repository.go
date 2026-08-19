package user

import (
	"context"
	"errors"
)

// ErrNotFound menandakan user tidak ditemukan di PostgreSQL.
var ErrNotFound = errors.New("user not found")

// ErrConflict menandakan nomor HP atau email sudah digunakan user lain.
var ErrConflict = errors.New("user data conflicts with another account")

// SettingsRepository menyimpan perubahan profil dan hash kredensial secara terisolasi.
type SettingsRepository interface {
	UpdateProfile(ctx context.Context, account *User) error
	UpdatePINHash(ctx context.Context, userID, hash string) error
	UpdatePasswordHash(ctx context.Context, userID, hash string) error
}

// Repository mendefinisikan akses data user yang diperlukan aplikasi.
type Repository interface {
	FindByIdentifier(ctx context.Context, identifier string) (*User, error)
	FindByID(ctx context.Context, id string) (*User, error)
}
