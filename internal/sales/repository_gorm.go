package sales

import (
	"context"
	"strings"

	"gorm.io/gorm"
)

// GORMRepository membaca daftar penjualan melalui PostgreSQL/GORM.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository membuat repository daftar penjualan berbasis GORM.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// ListOrders mengambil transaksi outlet beserta label jenis order dan meja.
func (repository *GORMRepository) ListOrders(ctx context.Context, filter OrderFilter) ([]Order, int64, error) {
	query := repository.db.WithContext(ctx).
		Table("transactions AS t").
		Joins("LEFT JOIN (SELECT transaction_id, outlet_id, SUM(total_amount) AS refunded FROM transaction_refunds GROUP BY transaction_id,outlet_id) rf ON rf.transaction_id=t.id AND rf.outlet_id=t.outlet_id").
		Joins("LEFT JOIN order_types AS ot ON ot.id = t.order_type_id AND ot.outlet_id = t.outlet_id").
		Joins("LEFT JOIN restaurant_tables AS rt ON rt.id = t.restaurant_table_id AND rt.outlet_id = t.outlet_id").
		Where("t.outlet_id = ?", filter.OutletID)
	if filter.Search != "" {
		pattern := "%" + strings.TrimSpace(filter.Search) + "%"
		query = query.Where("(t.transaction_number ILIKE ? OR t.customer_name ILIKE ?)", pattern, pattern)
	}
	if filter.OrderTypeID != "" {
		query = query.Where("t.order_type_id = ?", filter.OrderTypeID)
	}
	if filter.Status != "" {
		query = query.Where("t.status = ?", filter.Status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []Order{}, 0, nil
	}

	var values []Order
	err := query.Select(`t.id AS id, t.transaction_number AS receipt_number, t.customer_id AS customer_id,
		t.customer_name AS customer_name, t.paid_at AS ordered_at, ot.id AS order_type_id,
		ot.code AS order_type_code, ot.name AS order_type_name, rt.id AS restaurant_table_id,
		rt.name AS restaurant_table_name, t.total_quantity AS total_quantity,
		t.total_amount AS total_amount, t.status AS status, t.refund_revision AS refund_revision,
 CASE WHEN t.status IN ('refunded','cancelled') AND t.refund_revision=0 THEN NULL ELSE COALESCE(rf.refunded,0) END AS refunded_amount,
 CASE WHEN t.status IN ('refunded','cancelled') AND t.refund_revision=0 THEN NULL ELSE t.total_amount-COALESCE(rf.refunded,0) END AS net_after_refund,
 (t.status IN ('paid','partially_refunded') OR (t.status='refunded' AND t.refund_revision>0)) AS refunds_supported`).
		Order("t.paid_at DESC, t.transaction_number DESC").
		Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize).Scan(&values).Error
	return values, total, err
}

// ListCustomers mengambil pelanggan aktif outlet beserta label gender opsional.
func (repository *GORMRepository) ListCustomers(ctx context.Context, filter CustomerFilter) ([]Customer, int64, error) {
	query := repository.db.WithContext(ctx).
		Table("customers AS c").
		Joins("LEFT JOIN genders AS g ON g.id = c.gender_id").
		Where("c.outlet_id = ? AND c.is_active = ?", filter.OutletID, true)
	if filter.Search != "" {
		pattern := "%" + strings.TrimSpace(filter.Search) + "%"
		query = query.Where("(c.name ILIKE ? OR c.phone_number ILIKE ? OR c.email ILIKE ?)", pattern, pattern, pattern)
	}
	if filter.GenderID != "" {
		query = query.Where("c.gender_id = ?", filter.GenderID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []Customer{}, 0, nil
	}

	var values []Customer
	err := query.Select(`c.id AS id, c.name AS name, c.address AS address, c.points AS points,
		c.gender_id AS gender_id, g.name AS gender_name, c.phone_number AS phone_number,
		c.email AS email, c.created_at AS created_at, c.updated_at AS updated_at`).
		Order("c.name ASC, c.id ASC").Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize).Scan(&values).Error
	return values, total, err
}
