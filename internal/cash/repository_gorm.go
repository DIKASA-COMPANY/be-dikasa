package cash

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

const activeOutletConstraint = "cash_sessions_outlet_open_uidx"

// GORMRepository menyimpan dan mengambil sesi kas melalui PostgreSQL/GORM.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository membuat repository sesi kas berbasis GORM.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// Create menyimpan sesi kas dan memetakan konflik sesi aktif yang dijaga database.
func (repository *GORMRepository) Create(ctx context.Context, session *Session) error {
	err := repository.db.WithContext(ctx).Create(session).Error
	if isActiveSessionConflict(err) {
		return ErrActiveSessionExists
	}
	return err
}

// FindOpenByOutletID mengambil sesi kas aktif terbaru untuk satu outlet.
func (repository *GORMRepository) FindOpenByOutletID(ctx context.Context, outletID string) (*Session, error) {
	var session Session
	err := repository.db.WithContext(ctx).
		Where("outlet_id = ? AND status = ?", outletID, StatusOpen).
		Order("opened_at DESC").
		First(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &session, nil
}

func isActiveSessionConflict(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505" && postgresError.ConstraintName == activeOutletConstraint
}
