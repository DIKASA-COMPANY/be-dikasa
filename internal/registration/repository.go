package registration

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrConflict menandakan username, nomor telepon, atau email sudah terdaftar.
	ErrConflict = errors.New("registration conflict")
	// ErrNotFound menandakan registrasi tertunda tidak ditemukan.
	ErrNotFound = errors.New("pending registration not found")
	// ErrAlreadyVerified menandakan email sudah pernah dikonfirmasi.
	ErrAlreadyVerified = errors.New("registration already verified")
)

// CreatePendingParams adalah data atomik untuk membuat outlet, user nonaktif, dan OTP.
type CreatePendingParams struct {
	BusinessName      string
	BusinessAddress   string
	BusinessPhone     string
	BusinessEmail     *string
	OwnerName         string
	OwnerDateOfBirth  time.Time
	OwnerPhone        string
	OwnerEmail        string
	Username          string
	PasswordHash      string
	VerificationEmail string
	CodeHash          string
	ExpiresAt         time.Time
	ResendAvailableAt time.Time
	MaxAttempts       int
}

// ReplaceOTPParams adalah perubahan OTP ketika user meminta kode baru.
type ReplaceOTPParams struct {
	ID                string
	CodeHash          string
	ExpiresAt         time.Time
	ResendAvailableAt time.Time
	UpdatedAt         time.Time
}

// Repository mendefinisikan penyimpanan registrasi dan perubahan status verifikasi.
type Repository interface {
	// CreatePending membuat seluruh data registrasi dalam satu transaksi.
	CreatePending(ctx context.Context, params CreatePendingParams) (*Pending, error)
	// FindPendingByEmail mencari OTP registrasi berdasarkan email terpilih.
	FindPendingByEmail(ctx context.Context, email string) (*Pending, error)
	// IncrementFailedAttempts mencatat satu kode salah secara atomik.
	IncrementFailedAttempts(ctx context.Context, id string, maxAttempts int) (int, error)
	// MarkVerified mengaktifkan akun dan outlet serta menandai OTP terpakai.
	MarkVerified(ctx context.Context, id string, verifiedAt time.Time) error
	// ReplaceOTP mengganti hash dan timer OTP serta mereset percobaan.
	ReplaceOTP(ctx context.Context, params ReplaceOTPParams) error
}
