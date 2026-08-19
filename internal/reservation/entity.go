// Package reservation mengelola reservasi dan meja restoran per outlet.
package reservation

import "time"

const (
	// StatusBooked menandakan reservasi baru tercatat.
	StatusBooked = "booked"
	// StatusConfirmed menandakan reservasi sudah dikonfirmasi.
	StatusConfirmed = "confirmed"
	// StatusSeated menandakan pelanggan sudah mendapat meja.
	StatusSeated = "seated"
	// StatusCompleted menandakan layanan reservasi selesai.
	StatusCompleted = "completed"
	// StatusCancelled menandakan reservasi dibatalkan.
	StatusCancelled = "cancelled"
)

// Type adalah master global jenis reservasi.
type Type struct {
	ID        string    `gorm:"type:uuid;primaryKey"`
	Code      string    `gorm:"size:40;uniqueIndex;not null"`
	Name      string    `gorm:"size:100;not null"`
	SortOrder int       `gorm:"not null"`
	IsActive  bool      `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

// TableName menetapkan tabel master jenis reservasi.
func (Type) TableName() string { return "reservation_types" }

// Table adalah meja restoran pada satu outlet.
type Table struct {
	ID        string    `gorm:"type:uuid;primaryKey"`
	OutletID  string    `gorm:"type:uuid;not null;index"`
	Name      string    `gorm:"size:100;not null"`
	Capacity  int       `gorm:"not null"`
	SortOrder int       `gorm:"not null"`
	IsActive  bool      `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

// TableName menetapkan tabel meja restoran.
func (Table) TableName() string { return "restaurant_tables" }

// Reservation adalah pemesanan meja pada waktu tertentu.
type Reservation struct {
	ID                string    `gorm:"type:uuid;primaryKey"`
	OutletID          string    `gorm:"type:uuid;not null;index"`
	ReservationTypeID string    `gorm:"type:uuid;not null;index"`
	RestaurantTableID string    `gorm:"type:uuid;not null;index"`
	CreatedBy         string    `gorm:"type:uuid;not null;index"`
	CustomerName      string    `gorm:"size:100;not null"`
	CustomerContact   string    `gorm:"size:30;not null"`
	ReservedAt        time.Time `gorm:"not null;index"`
	Notes             string    `gorm:"type:text;not null"`
	PrintReservation  bool      `gorm:"not null"`
	SendReservation   bool      `gorm:"not null"`
	Status            string    `gorm:"size:20;not null"`
	IsActive          bool      `gorm:"not null"`
	CreatedAt         time.Time `gorm:"not null"`
	UpdatedAt         time.Time `gorm:"not null"`
	ReservationType   Type      `gorm:"foreignKey:ReservationTypeID"`
	RestaurantTable   Table     `gorm:"foreignKey:RestaurantTableID"`
}

// TableName menetapkan tabel reservasi.
func (Reservation) TableName() string { return "reservations" }

// Mutation adalah perubahan reservasi, dengan Fields untuk PATCH parsial.
type Mutation struct {
	CustomerName, CustomerContact, ReservationTypeID, RestaurantTableID, Notes, Status string
	ReservedAt                                                                         time.Time
	PrintReservation, SendReservation                                                  bool
	Fields                                                                             map[string]bool
}

// Filter mengendalikan list reservasi outlet.
type Filter struct {
	OutletID       string
	Date           string
	Status         string
	Page, PageSize int
}

// TableMutation adalah input konfigurasi meja.
type TableMutation struct {
	Name      string
	Capacity  int
	SortOrder int
	IsActive  bool
}
