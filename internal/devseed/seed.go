// Package devseed creates an explicit local development account without modifying existing users.
package devseed

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Run creates admin and its sample outlet/catalog atomically, or leaves an existing admin untouched.
// Callers must restrict this operation to local development databases.
func Run(db *gorm.DB, password string) error {
	if len(password) < 8 || len(password) > 72 {
		return fmt.Errorf("seed password must contain 8 to 72 bytes")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		// Serialize explicit seed runs, including the initial existence check.
		if err := tx.Exec("SELECT pg_advisory_xact_lock(729431)").Error; err != nil {
			return err
		}
		var exists bool
		if err := tx.Raw("SELECT EXISTS (SELECT 1 FROM users WHERE LOWER(username) = 'admin')").Scan(&exists).Error; err != nil {
			return err
		}
		if exists {
			return nil
		}
		result := tx.Exec(`WITH new_outlet AS (
    INSERT INTO outlets (name, business_type, address, phone_number, tax_percentage, is_active)
    VALUES ('Dikasa Local', 'restaurant', 'Local development', '080000000001', 10, TRUE) RETURNING id
  ), new_user AS (
    INSERT INTO users (role_id, outlet_id, username, phone_number, password_hash, first_name, last_name, is_active)
    SELECT roles.id, new_outlet.id, 'admin', '080000000001', ?, 'Local', 'Manager', TRUE
    FROM roles CROSS JOIN new_outlet WHERE roles.name = 'manager' RETURNING id
  ), new_category AS (
    INSERT INTO product_categories (outlet_id, product_type_id, name, sort_order, is_active)
    SELECT new_outlet.id, product_types.id, 'Menu Lokal', 10, TRUE
    FROM new_outlet CROSS JOIN product_types WHERE product_types.code = 'food' RETURNING id, outlet_id
  ), new_product AS (
    INSERT INTO products (outlet_id, category_id, name, price, stock_status, is_active)
    SELECT outlet_id, id, 'Nasi Goreng', 21000, 'available', TRUE FROM new_category RETURNING id
  ), new_order_types AS (
    INSERT INTO order_types (outlet_id, code, name, sort_order, is_active)
    SELECT id, 'takeaway', 'Bungkus', 10, TRUE FROM new_outlet RETURNING id
  )
  INSERT INTO payment_wallets (outlet_id, code, name, icon_key, sort_order, is_active)
  SELECT id, 'gopay', 'GoPay', 'gopay', 10, TRUE FROM new_outlet`, string(hash))
		return result.Error
	})
}
