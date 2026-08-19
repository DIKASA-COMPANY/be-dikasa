package reservation

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// GORMRepository menyimpan reservasi dengan PostgreSQL/GORM.
type GORMRepository struct{ db *gorm.DB }

// NewGORMRepository membuat repository reservasi PostgreSQL.
func NewGORMRepository(db *gorm.DB) *GORMRepository { return &GORMRepository{db: db} }

// ListTypes mengambil jenis reservasi aktif dengan urutan stabil.
func (repository *GORMRepository) ListTypes(ctx context.Context) ([]Type, error) {
	var values []Type
	err := repository.db.WithContext(ctx).Where("is_active = ?", true).Order("sort_order ASC, name ASC").Find(&values).Error
	return values, err
}

// ListTables mengambil meja aktif outlet yang tersedia pada waktu opsional.
func (repository *GORMRepository) ListTables(ctx context.Context, outletID string, reservedAt *time.Time) ([]Table, error) {
	var values []Table
	query := repository.db.WithContext(ctx).Where("outlet_id = ? AND is_active = ?", outletID, true)
	if reservedAt != nil {
		query = query.Where(`NOT EXISTS (SELECT 1 FROM reservations r WHERE r.outlet_id = restaurant_tables.outlet_id AND r.restaurant_table_id = restaurant_tables.id AND r.reserved_at = ? AND r.is_active = TRUE AND r.status <> 'cancelled')`, *reservedAt)
	}
	err := query.Order("sort_order ASC, name ASC").Find(&values).Error
	return values, err
}

// FindType mengambil jenis reservasi berdasarkan ID.
func (repository *GORMRepository) FindType(ctx context.Context, id string) (*Type, error) {
	var value Type
	err := repository.db.WithContext(ctx).Where("id = ?", id).First(&value).Error
	return &value, normalizeError(err)
}

// FindTable mengambil meja pada scope outlet.
func (repository *GORMRepository) FindTable(ctx context.Context, outletID, id string) (*Table, error) {
	var value Table
	err := repository.db.WithContext(ctx).Where("outlet_id = ? AND id = ?", outletID, id).First(&value).Error
	return &value, normalizeError(err)
}

// CreateTable menyimpan meja baru.
func (repository *GORMRepository) CreateTable(ctx context.Context, value *Table) error {
	return normalizeError(repository.db.WithContext(ctx).Create(value).Error)
}

// UpdateTable memperbarui meja dalam scope outlet.
func (repository *GORMRepository) UpdateTable(ctx context.Context, value *Table) error {
	result := repository.db.WithContext(ctx).Model(&Table{}).Where("id = ? AND outlet_id = ?", value.ID, value.OutletID).
		Updates(map[string]any{"name": value.Name, "capacity": value.Capacity, "sort_order": value.SortOrder, "is_active": value.IsActive})
	if result.Error != nil {
		return normalizeError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// SoftDeleteTable menonaktifkan meja.
func (repository *GORMRepository) SoftDeleteTable(ctx context.Context, outletID, id string) error {
	result := repository.db.WithContext(ctx).Model(&Table{}).Where("id = ? AND outlet_id = ? AND is_active = ?", id, outletID, true).Update("is_active", false)
	if result.Error != nil {
		return normalizeError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// HasScheduleConflict memeriksa pemakaian meja aktif pada waktu persis yang sama.
func (repository *GORMRepository) HasScheduleConflict(ctx context.Context, outletID, tableID string, reservedAt time.Time, excludeID string) (bool, error) {
	query := repository.db.WithContext(ctx).Model(&Reservation{}).Where("outlet_id = ? AND restaurant_table_id = ? AND reserved_at = ? AND is_active = ? AND status <> ?", outletID, tableID, reservedAt, true, StatusCancelled)
	if excludeID != "" {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	err := query.Count(&count).Error
	return count > 0, err
}

// List mengambil halaman reservasi dan total hasil.
func (repository *GORMRepository) List(ctx context.Context, filter Filter) ([]Reservation, int64, error) {
	query := repository.db.WithContext(ctx).Model(&Reservation{}).Where("outlet_id = ? AND is_active = ?", filter.OutletID, true)
	if filter.Date != "" {
		start, _ := time.Parse("2006-01-02", filter.Date)
		query = query.Where("reserved_at >= ? AND reserved_at < ?", start, start.AddDate(0, 0, 1))
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []Reservation{}, 0, nil
	}
	var values []Reservation
	err := query.Preload("ReservationType").Preload("RestaurantTable").Order("reserved_at ASC, customer_name ASC").
		Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize).Find(&values).Error
	return values, total, err
}

// Find mengambil detail reservasi pada scope outlet.
func (repository *GORMRepository) Find(ctx context.Context, outletID, id string) (*Reservation, error) {
	var value Reservation
	err := repository.db.WithContext(ctx).Preload("ReservationType").Preload("RestaurantTable").
		Where("outlet_id = ? AND id = ? AND is_active = ?", outletID, id, true).First(&value).Error
	return &value, normalizeError(err)
}

// Create menyimpan reservasi baru.
func (repository *GORMRepository) Create(ctx context.Context, value *Reservation) error {
	return normalizeError(repository.db.WithContext(ctx).Omit("ReservationType", "RestaurantTable").Create(value).Error)
}

// Update memperbarui field reservasi yang dapat dikelola client.
func (repository *GORMRepository) Update(ctx context.Context, value *Reservation) error {
	result := repository.db.WithContext(ctx).Model(&Reservation{}).Where("id = ? AND outlet_id = ?", value.ID, value.OutletID).Updates(map[string]any{
		"reservation_type_id": value.ReservationTypeID, "restaurant_table_id": value.RestaurantTableID,
		"customer_name": value.CustomerName, "customer_contact": value.CustomerContact, "reserved_at": value.ReservedAt,
		"notes": value.Notes, "print_reservation": value.PrintReservation, "send_reservation": value.SendReservation,
		"status": value.Status,
	})
	if result.Error != nil {
		return normalizeError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// Cancel mengubah status menjadi cancelled dan melakukan soft delete.
func (repository *GORMRepository) Cancel(ctx context.Context, outletID, id string) error {
	result := repository.db.WithContext(ctx).Model(&Reservation{}).Where("id = ? AND outlet_id = ? AND is_active = ?", id, outletID, true).
		Updates(map[string]any{"status": StatusCancelled, "is_active": false})
	if result.Error != nil {
		return normalizeError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func normalizeError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && (postgresError.Code == "23505" || postgresError.Code == "23503") {
		return ErrConflict
	}
	return err
}
