DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM transaction_refunds) OR EXISTS(SELECT 1 FROM transactions WHERE refund_revision>0 OR status='partially_refunded') THEN
 RAISE EXCEPTION 'Refusing to erase refund history'; END IF;
END $$;
ALTER TABLE stock_movements DROP CONSTRAINT stock_movements_refund_scope_fk, DROP CONSTRAINT stock_movements_refund_kind_check, DROP CONSTRAINT stock_movements_kind_check;
ALTER TABLE stock_movements DROP COLUMN refund_id;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_kind_check CHECK(kind IN ('configure','adjust','sale'));
DROP TABLE transaction_refund_items;
DROP TABLE transaction_refunds;
ALTER TABLE transaction_items DROP CONSTRAINT transaction_items_refund_scope_unique;
ALTER TABLE transactions DROP CONSTRAINT transactions_status_check;
ALTER TABLE transactions ADD CONSTRAINT transactions_status_check CHECK(status IN ('paid','refunded','cancelled'));
ALTER TABLE transactions DROP COLUMN refund_revision;
