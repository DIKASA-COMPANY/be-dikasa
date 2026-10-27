ALTER TABLE transactions ADD COLUMN refund_revision BIGINT NOT NULL DEFAULT 0 CHECK(refund_revision >= 0);
ALTER TABLE transactions DROP CONSTRAINT transactions_status_check;
ALTER TABLE transactions ADD CONSTRAINT transactions_status_check CHECK(status IN ('paid','partially_refunded','refunded','cancelled'));
ALTER TABLE transaction_items ADD CONSTRAINT transaction_items_refund_scope_unique UNIQUE(id,transaction_id,outlet_id);
CREATE TABLE transaction_refunds (
 id UUID PRIMARY KEY, number VARCHAR(50) NOT NULL UNIQUE,
 outlet_id UUID NOT NULL REFERENCES outlets(id), transaction_id UUID NOT NULL,
 actor_id UUID NOT NULL REFERENCES users(id), actor_name TEXT NOT NULL,
 cash_session_id UUID NOT NULL, reason VARCHAR(500) NOT NULL CHECK(length(trim(reason))>0),
 method VARCHAR(20) NOT NULL CHECK(method IN ('cash','wallet','debit_card','qris','transfer')),
 mode VARCHAR(10) NOT NULL CHECK(mode IN ('full','items')),
 net_amount BIGINT NOT NULL CHECK(net_amount>=0), tax_amount BIGINT NOT NULL CHECK(tax_amount>=0),
 total_amount BIGINT NOT NULL CHECK(total_amount BETWEEN 0 AND 9007199254740991),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), request_id UUID NOT NULL,
 payload_hash CHAR(64) NOT NULL, response JSONB NOT NULL DEFAULT '{}',
 CHECK(total_amount=net_amount+tax_amount), UNIQUE(outlet_id,request_id), UNIQUE(id,transaction_id,outlet_id),
 FOREIGN KEY(transaction_id,outlet_id) REFERENCES transactions(id,outlet_id),
 FOREIGN KEY(cash_session_id,outlet_id) REFERENCES cash_sessions(id,outlet_id)
);
CREATE INDEX transaction_refunds_receipt_time_idx ON transaction_refunds(outlet_id,transaction_id,created_at DESC,id);
CREATE INDEX transaction_refunds_outlet_time_idx ON transaction_refunds(outlet_id,created_at DESC,id);
CREATE TABLE transaction_refund_items (
 id UUID PRIMARY KEY, refund_id UUID NOT NULL, transaction_id UUID NOT NULL, outlet_id UUID NOT NULL,
 transaction_item_id UUID NOT NULL, quantity INTEGER NOT NULL CHECK(quantity BETWEEN 1 AND 1000),
 restock_quantity INTEGER NOT NULL CHECK(restock_quantity BETWEEN 0 AND quantity),
 net_amount BIGINT NOT NULL CHECK(net_amount>=0), tax_amount BIGINT NOT NULL CHECK(tax_amount>=0),
 total_amount BIGINT NOT NULL CHECK(total_amount BETWEEN 0 AND 9007199254740991),
 CHECK(total_amount=net_amount+tax_amount), UNIQUE(refund_id,transaction_item_id),
 FOREIGN KEY(refund_id,transaction_id,outlet_id) REFERENCES transaction_refunds(id,transaction_id,outlet_id),
 FOREIGN KEY(transaction_item_id,transaction_id,outlet_id) REFERENCES transaction_items(id,transaction_id,outlet_id)
);
CREATE INDEX transaction_refund_items_receipt_idx ON transaction_refund_items(outlet_id,transaction_id,transaction_item_id);
ALTER TABLE stock_movements ADD COLUMN refund_id UUID;
ALTER TABLE stock_movements DROP CONSTRAINT stock_movements_kind_check;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_kind_check CHECK(kind IN ('configure','adjust','sale','refund_return'));
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_refund_kind_check CHECK((kind='refund_return')=(refund_id IS NOT NULL));
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_refund_scope_fk FOREIGN KEY(refund_id,transaction_id,outlet_id) REFERENCES transaction_refunds(id,transaction_id,outlet_id);
