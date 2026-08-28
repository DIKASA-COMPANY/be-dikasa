package payment

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// GORMRepository mengambil konfigurasi pembayaran melalui PostgreSQL/GORM.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository membuat repository pembayaran berbasis GORM.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// FindTaxPercentageByOutletID mengambil persentase pajak terbaru dari outlet aktif.
func (repository *GORMRepository) FindTaxPercentageByOutletID(ctx context.Context, outletID string) (float64, error) {
	var row struct {
		TaxPercentage float64 `gorm:"column:tax_percentage"`
	}
	err := repository.db.WithContext(ctx).
		Table("outlets").
		Select("tax_percentage").
		Where("id = ? AND is_active = ?", outletID, true).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrOutletNotFound
		}
		return 0, err
	}
	return row.TaxPercentage, nil
}

// ListWallets mengambil wallet outlet dengan urutan dropdown yang stabil.
func (repository *GORMRepository) ListWallets(ctx context.Context, outletID string, includeInactive bool) ([]Wallet, error) {
	var wallets []Wallet
	query := repository.db.WithContext(ctx).Where("outlet_id = ?", outletID)
	if !includeInactive {
		query = query.Where("is_active = ?", true)
	}
	err := query.Order("sort_order ASC, name ASC").Find(&wallets).Error
	return wallets, err
}

// FindWallet mengambil wallet pada scope outlet.
func (repository *GORMRepository) FindWallet(ctx context.Context, outletID, walletID string, includeInactive bool) (*Wallet, error) {
	var wallet Wallet
	query := repository.db.WithContext(ctx).Where("outlet_id = ? AND id = ?", outletID, walletID)
	if !includeInactive {
		query = query.Where("is_active = ?", true)
	}
	err := query.First(&wallet).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrWalletNotFound
	}
	return &wallet, err
}

// CreateWallet menyimpan wallet outlet baru.
func (repository *GORMRepository) CreateWallet(ctx context.Context, wallet *Wallet) error {
	return normalizePaymentWriteError(repository.db.WithContext(ctx).Create(wallet).Error)
}

// UpdateWallet memperbarui wallet dalam scope outlet.
func (repository *GORMRepository) UpdateWallet(ctx context.Context, wallet *Wallet) error {
	result := repository.db.WithContext(ctx).Model(&Wallet{}).Where("id = ? AND outlet_id = ?", wallet.ID, wallet.OutletID).
		Updates(map[string]any{"code": wallet.Code, "name": wallet.Name, "icon_key": wallet.IconKey, "sort_order": wallet.SortOrder, "is_active": wallet.IsActive})
	if result.Error != nil {
		return normalizePaymentWriteError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrWalletNotFound
	}
	return nil
}

// SoftDeleteWallet menonaktifkan wallet tanpa menghapus riwayat transaksi.
func (repository *GORMRepository) SoftDeleteWallet(ctx context.Context, outletID, walletID string) error {
	result := repository.db.WithContext(ctx).Model(&Wallet{}).
		Where("id = ? AND outlet_id = ? AND is_active = ?", walletID, outletID, true).
		Update("is_active", false)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrWalletNotFound
	}
	return nil
}

// FindOpenCashSessionIDByOutletID mengambil sesi kas aktif terbaru untuk transaksi.
func (repository *GORMRepository) FindOpenCashSessionIDByOutletID(ctx context.Context, outletID string) (string, error) {
	var row struct {
		ID string `gorm:"column:id"`
	}
	err := repository.db.WithContext(ctx).Table("cash_sessions").Select("id").
		Where("outlet_id = ? AND status = ?", outletID, "open").Order("opened_at DESC").Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", ErrCashSessionNotFound
	}
	return row.ID, err
}

// CreateTransaction menyimpan header dan seluruh item dalam satu transaksi database.
func (repository *GORMRepository) CreateTransaction(ctx context.Context, value *Transaction) error {
	return repository.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if err := transaction.Omit("Items", "Wallet").Create(value).Error; err != nil {
			return normalizePaymentWriteError(err)
		}
		for index := range value.Items {
			value.Items[index].TransactionID = value.ID
			value.Items[index].OutletID = value.OutletID
			if err := transaction.Create(&value.Items[index]).Error; err != nil {
				return normalizePaymentWriteError(err)
			}
		}
		return nil
	})
}

func normalizePaymentWriteError(err error) error {
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) {
		return err
	}
	switch postgresError.Code {
	case "23505":
		return ErrConflict
	case "23503":
		return ErrInvalidReference
	case "23514":
		return ErrValidation
	default:
		return err
	}
}
