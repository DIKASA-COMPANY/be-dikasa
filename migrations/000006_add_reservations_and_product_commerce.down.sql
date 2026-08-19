DROP TABLE IF EXISTS reservations;
DROP TABLE IF EXISTS restaurant_tables;
DROP TABLE IF EXISTS reservation_types;
DROP TABLE IF EXISTS product_variants;

ALTER TABLE products
    DROP CONSTRAINT IF EXISTS products_id_outlet_unique,
    DROP CONSTRAINT IF EXISTS products_online_cost_price_check,
    DROP CONSTRAINT IF EXISTS products_cost_price_check,
    DROP CONSTRAINT IF EXISTS products_online_selling_price_check,
    DROP CONSTRAINT IF EXISTS products_stock_status_check,
    DROP COLUMN IF EXISTS online_cost_price,
    DROP COLUMN IF EXISTS cost_price,
    DROP COLUMN IF EXISTS online_selling_price,
    DROP COLUMN IF EXISTS stock_status;
