package user

import (
	"context"
	"errors"
)

// ErrNotFound menandakan user tidak ditemukan di PostgreSQL.
var ErrNotFound = errors.New("user not found")

// Repository mendefinisikan akses data user yang diperlukan aplikasi.
type Repository interface {
	FindByIdentifier(ctx context.Context, identifier string) (*User, error)
	FindByID(ctx context.Context, id string) (*User, error)
}
