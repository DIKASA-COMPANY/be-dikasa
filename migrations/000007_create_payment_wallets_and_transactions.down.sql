DROP TABLE IF EXISTS transaction_items;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS payment_wallets;

ALTER TABLE product_variants
    DROP CONSTRAINT IF EXISTS product_variants_id_product_outlet_unique;
ALTER TABLE order_types
    DROP CONSTRAINT IF EXISTS order_types_id_outlet_unique;
ALTER TABLE cash_sessions
    DROP CONSTRAINT IF EXISTS cash_sessions_id_outlet_unique;

