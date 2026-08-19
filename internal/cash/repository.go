package cash

import (
	"context"
	"errors"
)

var (
	// ErrNotFound menandakan tidak ada sesi kas aktif untuk outlet.
	ErrNotFound = errors.New("open cash session not found")
	// ErrActiveSessionExists menandakan outlet sudah memiliki sesi kas aktif.
	ErrActiveSessionExists = errors.New("active cash session already exists")
)

// Repository mendefinisikan penyimpanan sesi kas yang dibutuhkan use case.
type Repository interface {
	Create(ctx context.Context, session *Session) error
	FindOpenByOutletID(ctx context.Context, outletID string) (*Session, error)
}
