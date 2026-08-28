DROP INDEX IF EXISTS transactions_customer_id_idx;

ALTER TABLE transactions
    DROP CONSTRAINT IF EXISTS transactions_customer_outlet_fk,
    DROP COLUMN IF EXISTS customer_name,
    DROP COLUMN IF EXISTS customer_id;

DROP TABLE IF EXISTS customers;
