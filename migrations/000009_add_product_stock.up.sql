ALTER TABLE products ADD COLUMN track_stock BOOLEAN NOT NULL DEFAULT FALSE,
 ADD COLUMN stock_quantity BIGINT NOT NULL DEFAULT 0 CHECK (stock_quantity BETWEEN 0 AND 1000000000),
 ADD COLUMN low_stock_threshold BIGINT NOT NULL DEFAULT 0 CHECK (low_stock_threshold BETWEEN 0 AND 1000000000);
CREATE TABLE stock_movements (
 id UUID PRIMARY KEY,
 outlet_id UUID NOT NULL REFERENCES outlets(id),
 product_id UUID NOT NULL,
 transaction_id UUID REFERENCES transactions(id),
 created_by UUID NOT NULL REFERENCES users(id),
 request_id UUID,
 payload_hash VARCHAR(64) NOT NULL DEFAULT '',
 kind VARCHAR(20) NOT NULL CHECK (kind IN ('configure','adjust','sale')),
 quantity_delta BIGINT NOT NULL,
 balance_after BIGINT NOT NULL CHECK (balance_after >= 0),
 track_stock BOOLEAN NOT NULL,
 low_stock_threshold BIGINT NOT NULL,
 reason TEXT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(outlet_id,request_id),
 FOREIGN KEY(product_id,outlet_id) REFERENCES products(id,outlet_id)
);
CREATE INDEX stock_movements_product_time_idx ON stock_movements(outlet_id,product_id,created_at DESC);
