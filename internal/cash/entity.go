// Package cash mengelola sesi kas outlet, termasuk kas awal per shift operasional.
package cash

import "time"

const (
	// StatusOpen menandakan sesi kas masih aktif.
	StatusOpen = "open"
	// StatusClosed menandakan sesi kas telah ditutup.
	StatusClosed = "closed"
)

// Session adalah periode operasional kas pada satu outlet.
type Session struct {
	ID          string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	OutletID    string    `gorm:"type:uuid;not null;index"`
	UserID      string    `gorm:"type:uuid;not null;index"`
	OpeningCash int64     `gorm:"type:bigint;not null"`
	ClosingCash *int64    `gorm:"type:bigint"`
	Status      string    `gorm:"size:20;not null"`
	OpenedAt    time.Time `gorm:"not null"`
	ClosedAt    *time.Time
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

// TableName menetapkan nama tabel PostgreSQL untuk Session.
func (Session) TableName() string { return "cash_sessions" }
