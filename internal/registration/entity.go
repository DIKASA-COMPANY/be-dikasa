// Package registration mengelola pendaftaran outlet, akun pemilik, dan verifikasi email dengan OTP.
package registration

import "time"

// Registration memuat data usaha, pemilik, dan akun yang akan didaftarkan.
type Registration struct {
	BusinessName      string
	BusinessAddress   string
	BusinessPhone     string
	BusinessEmail     *string
	OwnerName         string
	OwnerDateOfBirth  string
	OwnerPhone        string
	OwnerEmail        string
	Username          string
	Password          string
	VerificationEmail string
}

// Pending adalah status OTP dari akun yang belum menyelesaikan verifikasi email.
type Pending struct {
	ID                string    `gorm:"type:uuid;primaryKey"`
	UserID            string    `gorm:"type:uuid;not null;uniqueIndex"`
	OutletID          string    `gorm:"type:uuid;not null;index"`
	Email             string    `gorm:"size:254;not null;uniqueIndex"`
	CodeHash          string    `gorm:"size:64;not null"`
	ExpiresAt         time.Time `gorm:"not null"`
	ResendAvailableAt time.Time `gorm:"not null"`
	Attempts          int       `gorm:"not null;default:0"`
	MaxAttempts       int       `gorm:"not null;default:5"`
	VerifiedAt        *time.Time
	CreatedAt         time.Time `gorm:"not null"`
	UpdatedAt         time.Time `gorm:"not null"`
}

// TableName menetapkan tabel PostgreSQL untuk OTP registrasi.
func (Pending) TableName() string { return "registration_otps" }

// Result memuat metadata aman yang diperlukan layar OTP.
type Result struct {
	Email             string
	OTPExpiresIn      int
	ResendAvailableIn int
}
