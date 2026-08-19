package registration

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"be-dikasa/internal/user"
)

// GORMRepository menyimpan registrasi melalui transaksi PostgreSQL/GORM.
type GORMRepository struct{ db *gorm.DB }

// NewGORMRepository membuat repository registrasi berbasis GORM.
func NewGORMRepository(db *gorm.DB) *GORMRepository { return &GORMRepository{db: db} }

// CreatePending membuat outlet, user nonaktif, dan OTP dalam satu transaksi.
func (repository *GORMRepository) CreatePending(ctx context.Context, params CreatePendingParams) (*Pending, error) {
	var pending Pending
	err := repository.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var role user.Role
		if err := transaction.Where("LOWER(name) = ?", "manager").First(&role).Error; err != nil {
			return err
		}
		outletID := uuid.NewString()
		now := time.Now().UTC()
		if err := transaction.Table("outlets").Create(map[string]any{
			"id": outletID, "name": params.BusinessName, "business_type": "restaurant", "address": params.BusinessAddress,
			"phone_number": params.BusinessPhone, "email": params.BusinessEmail, "tax_percentage": 0,
			"is_active": false, "created_at": now, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		userID := uuid.NewString()
		if err := transaction.Table("users").Create(map[string]any{
			"id": userID, "role_id": role.ID, "outlet_id": outletID, "username": params.Username,
			"phone_number": params.OwnerPhone, "email": params.OwnerEmail, "password_hash": params.PasswordHash,
			"first_name": params.OwnerName, "last_name": "", "date_of_birth": params.OwnerDateOfBirth,
			"is_active": false, "created_at": now, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		if err := transaction.Exec(`INSERT INTO order_types (outlet_id, code, name, sort_order)
			VALUES (?, 'free_table', 'Bebas Pilih Meja', 10),
			       (?, 'table_order', 'Pesan Meja', 20),
			       (?, 'takeaway', 'Bungkus', 30),
			       (?, 'delivery', 'Pesan Antar', 40),
			       (?, 'reservation', 'Reservasi', 50),
			       (?, 'online_delivery', 'Ojek Online', 60)`,
			outletID, outletID, outletID, outletID, outletID, outletID).Error; err != nil {
			return err
		}
		pending = Pending{
			ID: uuid.NewString(), UserID: userID, OutletID: outletID, Email: params.VerificationEmail,
			CodeHash: params.CodeHash, ExpiresAt: params.ExpiresAt, ResendAvailableAt: params.ResendAvailableAt,
			MaxAttempts: params.MaxAttempts,
		}
		return transaction.Create(&pending).Error
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrConflict
		}
		return nil, err
	}
	return &pending, nil
}

// FindPendingByEmail mengambil status OTP berdasarkan email tanpa membedakan kapitalisasi.
func (repository *GORMRepository) FindPendingByEmail(ctx context.Context, email string) (*Pending, error) {
	var pending Pending
	err := repository.db.WithContext(ctx).Where("LOWER(email) = LOWER(?)", strings.TrimSpace(email)).First(&pending).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &pending, nil
}

// IncrementFailedAttempts menambah percobaan gagal secara atomik sampai batas maksimum.
func (repository *GORMRepository) IncrementFailedAttempts(ctx context.Context, id string, maxAttempts int) (int, error) {
	result := repository.db.WithContext(ctx).Model(&Pending{}).
		Where("id = ? AND verified_at IS NULL AND attempts < ?", id, maxAttempts).
		UpdateColumn("attempts", gorm.Expr("attempts + 1"))
	if result.Error != nil {
		return 0, result.Error
	}
	var pending Pending
	if err := repository.db.WithContext(ctx).Select("attempts").First(&pending, "id = ?", id).Error; err != nil {
		return 0, err
	}
	return pending.Attempts, nil
}

// MarkVerified menandai OTP terpakai lalu mengaktifkan user dan outlet dalam satu transaksi.
func (repository *GORMRepository) MarkVerified(ctx context.Context, id string, verifiedAt time.Time) error {
	return repository.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var pending Pending
		if err := transaction.First(&pending, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		result := transaction.Model(&Pending{}).Where("id = ? AND verified_at IS NULL", id).
			Updates(map[string]any{"verified_at": verifiedAt, "updated_at": verifiedAt})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrAlreadyVerified
		}
		if err := transaction.Model(&user.User{}).Where("id = ?", pending.UserID).Updates(map[string]any{"is_active": true, "updated_at": verifiedAt}).Error; err != nil {
			return err
		}
		return transaction.Model(&user.Outlet{}).Where("id = ?", pending.OutletID).Updates(map[string]any{"is_active": true, "updated_at": verifiedAt}).Error
	})
}

// ReplaceOTP menginvalidasi kode lama, mereset percobaan, dan menyimpan timer baru.
func (repository *GORMRepository) ReplaceOTP(ctx context.Context, params ReplaceOTPParams) error {
	result := repository.db.WithContext(ctx).Model(&Pending{}).Where("id = ? AND verified_at IS NULL", params.ID).
		Updates(map[string]any{
			"code_hash": params.CodeHash, "expires_at": params.ExpiresAt, "resend_available_at": params.ResendAvailableAt,
			"attempts": 0, "updated_at": params.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// isUniqueViolation menyamakan error constraint PostgreSQL/GORM menjadi konflik domain yang aman.
func isUniqueViolation(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "sqlstate 23505") || strings.Contains(message, "duplicate key") || errors.Is(err, gorm.ErrDuplicatedKey)
}
