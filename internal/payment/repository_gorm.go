package payment

import (
	"be-dikasa/internal/catalog"
	"be-dikasa/internal/savedorder"
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
		var draft *savedorder.Order
		if value.SavedOrderID != nil {
			items := make([]savedorder.Item, 0, len(value.Items))
			for _, i := range value.Items {
				items = append(items, savedorder.Item{ProductID: i.ProductID, ProductVariantID: i.ProductVariantID, ProductName: i.ProductName, VariantName: i.VariantName, UnitPrice: i.UnitPrice, Quantity: i.Quantity, Notes: i.Notes})
			}
			var err error
			draft, err = savedorder.LockForCheckout(ctx, transaction, value.OutletID, *value.SavedOrderID, *value.SavedOrderRevision, value.OrderTypeID, items)
			if err != nil {
				return err
			}
		}

		// Hold the session through commit so closure cannot miss an in-flight sale.
		var session struct{ ID string }
		if err := transaction.Table("cash_sessions").Select("id").Clauses(clause.Locking{Strength: "SHARE"}).Where("id = ? AND outlet_id = ? AND status = 'open'", value.CashSessionID, value.OutletID).Take(&session).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCashSessionNotFound
			}
			return err
		}
		if err := transaction.Omit("Items", "Wallet").Create(value).Error; err != nil {
			return normalizePaymentWriteError(err)
		}
		stockItems := make([]catalog.StockSaleItem, 0, len(value.Items))
		for _, item := range value.Items {
			stockItems = append(stockItems, catalog.StockSaleItem{ProductID: item.ProductID, VariantID: item.ProductVariantID, UnitPrice: item.UnitPrice, Quantity: item.Quantity})
		}
		if err := catalog.DeductSaleStock(ctx, transaction, value.OutletID, value.CreatedBy, value.ID, stockItems); err != nil {
			return err
		}
		categories, err := catalog.CaptureSaleCategories(ctx, transaction, value.OutletID, stockItems)
		if err != nil {
			return err
		}
		for index := range value.Items {
			category := categories[value.Items[index].ProductID]
			value.Items[index].CategoryIDSnapshot = &category.ID
			value.Items[index].CategoryNameSnapshot = &category.Name

			value.Items[index].TransactionID = value.ID
			value.Items[index].OutletID = value.OutletID
			if err := transaction.Create(&value.Items[index]).Error; err != nil {
				return normalizePaymentWriteError(err)
			}
		}
		if draft != nil {
			return savedorder.CompleteCheckout(ctx, transaction, draft, value.ID)
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
