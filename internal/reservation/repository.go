package reservation

import (
	"context"
	"errors"
	"time"

	"be-dikasa/internal/user"
)

var (
	// ErrNotFound menandakan data tidak ada dalam scope outlet.
	ErrNotFound = errors.New("reservation data not found")
	// ErrForbidden menandakan user tidak memiliki outlet atau role yang sesuai.
	ErrForbidden = errors.New("reservation access is forbidden")
	// ErrValidation menandakan input reservasi tidak valid.
	ErrValidation = errors.New("reservation input is invalid")
	// ErrConflict menandakan constraint persistence bertabrakan.
	ErrConflict = errors.New("reservation data conflict")
	// ErrScheduleConflict menandakan meja sudah digunakan pada waktu yang sama.
	ErrScheduleConflict = errors.New("restaurant table is already reserved at that time")
)

// AccountFinder mengambil scope outlet dan role user.
type AccountFinder interface {
	FindByIDWithRole(context.Context, string) (*user.User, error)
}

// Repository mendefinisikan persistence reservasi.
type Repository interface {
	ListTypes(context.Context) ([]Type, error)
	ListTables(context.Context, string, *time.Time) ([]Table, error)
	FindType(context.Context, string) (*Type, error)
	FindTable(context.Context, string, string) (*Table, error)
	CreateTable(context.Context, *Table) error
	UpdateTable(context.Context, *Table) error
	SoftDeleteTable(context.Context, string, string) error
	HasScheduleConflict(context.Context, string, string, time.Time, string) (bool, error)
	List(context.Context, Filter) ([]Reservation, int64, error)
	Find(context.Context, string, string) (*Reservation, error)
	Create(context.Context, *Reservation) error
	Update(context.Context, *Reservation) error
	Cancel(context.Context, string, string) error
}
