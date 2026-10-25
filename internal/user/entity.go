// Package user mengelola user PostgreSQL dan profil operasional Dikasa.
package user

import "time"

// Role adalah peran akses user.
type Role struct {
	ID        string    `gorm:"type:uuid;primaryKey"`
	Name      string    `gorm:"size:50;uniqueIndex;not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

// TableName menetapkan nama tabel PostgreSQL untuk Role.
func (Role) TableName() string { return "roles" }

// Gender adalah data referensi gender user.
type Gender struct {
	ID        string    `gorm:"type:uuid;primaryKey"`
	Name      string    `gorm:"size:30;uniqueIndex;not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

// TableName menetapkan nama tabel PostgreSQL untuk Gender.
func (Gender) TableName() string { return "genders" }

// Outlet adalah unit usaha tempat user bekerja.
type Outlet struct {
	ID            string    `gorm:"type:uuid;primaryKey"`
	Name          string    `gorm:"size:120;not null"`
	BusinessType  string    `gorm:"size:60;not null"`
	Address       string    `gorm:"type:text;not null"`
	PhoneNumber   string    `gorm:"size:30;not null"`
	Email         *string   `gorm:"size:254;uniqueIndex"`
	TaxPercentage float64   `gorm:"type:numeric(5,2);not null"`
	IsActive      bool      `gorm:"not null;default:true"`
	CreatedAt     time.Time `gorm:"not null"`
	UpdatedAt     time.Time `gorm:"not null"`
}

// TableName menetapkan nama tabel PostgreSQL untuk Outlet.
func (Outlet) TableName() string { return "outlets" }

// User adalah akun lokal yang menjadi sumber kredensial login Dikasa.
type User struct {
	ID               string     `gorm:"type:uuid;primaryKey"`
	RoleID           string     `gorm:"type:uuid;not null;index"`
	GenderID         *string    `gorm:"type:uuid;index"`
	OutletID         *string    `gorm:"type:uuid;index"`
	Username         string     `gorm:"size:50;uniqueIndex;not null"`
	PhoneNumber      string     `gorm:"size:30;uniqueIndex;not null"`
	Email            *string    `gorm:"size:254;uniqueIndex"`
	PasswordHash     string     `gorm:"size:255;not null"`
	PINHash          *string    `gorm:"column:pin_hash;size:255"`
	AvatarURL        *string    `gorm:"type:text"`
	AvatarObjectName *string    `gorm:"type:text"`
	FirstName        string     `gorm:"size:100;not null"`
	LastName         string     `gorm:"size:100;not null"`
	PersonalAddress  *string    `gorm:"type:text"`
	DateOfBirth      *time.Time `gorm:"type:date"`
	IsActive         bool       `gorm:"not null;default:true"`
	CreatedAt        time.Time  `gorm:"not null"`
	UpdatedAt        time.Time  `gorm:"not null"`
	Role             Role       `gorm:"foreignKey:RoleID"`
	Gender           *Gender    `gorm:"foreignKey:GenderID"`
	Outlet           *Outlet    `gorm:"foreignKey:OutletID"`
}

// TableName menetapkan nama tabel PostgreSQL untuk User.
func (User) TableName() string { return "users" }
