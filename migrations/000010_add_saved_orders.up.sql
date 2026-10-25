CREATE TABLE saved_orders (
 id UUID PRIMARY KEY,
 outlet_id UUID NOT NULL REFERENCES outlets(id),
 created_by UUID NOT NULL REFERENCES users(id),
 label VARCHAR(100) NOT NULL,
 order_type_id UUID NOT NULL,
 revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0),
 status VARCHAR(10) NOT NULL DEFAULT 'open' CHECK (status IN ('open','paid','deleted')),
 transaction_id UUID UNIQUE REFERENCES transactions(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 CHECK ((status = 'paid') = (transaction_id IS NOT NULL)),
 UNIQUE (id,outlet_id),
 FOREIGN KEY(order_type_id,outlet_id) REFERENCES order_types(id,outlet_id)
);
CREATE INDEX saved_orders_outlet_status_updated_idx ON saved_orders(outlet_id,status,updated_at DESC,id);
CREATE TABLE saved_order_items (
 id UUID PRIMARY KEY,
 saved_order_id UUID NOT NULL,
 outlet_id UUID NOT NULL,
 position INTEGER NOT NULL CHECK(position >= 0),
 product_id UUID NOT NULL REFERENCES products(id),
 product_variant_id UUID,
 product_name VARCHAR(150) NOT NULL,
 variant_name VARCHAR(100),
 unit_price BIGINT NOT NULL CHECK(unit_price BETWEEN 0 AND 9007199254740991),
 quantity INTEGER NOT NULL CHECK(quantity BETWEEN 1 AND 1000),
 notes VARCHAR(500) NOT NULL DEFAULT '',
 UNIQUE(saved_order_id,position),
 FOREIGN KEY(saved_order_id,outlet_id) REFERENCES saved_orders(id,outlet_id),
 FOREIGN KEY(product_id,outlet_id) REFERENCES products(id,outlet_id),
 FOREIGN KEY(product_variant_id,product_id,outlet_id) REFERENCES product_variants(id,product_id,outlet_id)
);
CREATE TABLE saved_order_commands (
 outlet_id UUID NOT NULL REFERENCES outlets(id),
 request_id UUID NOT NULL,
 actor_id UUID NOT NULL REFERENCES users(id),
 operation VARCHAR(10) NOT NULL CHECK(operation IN ('create','update','delete')),
 payload_hash CHAR(64) NOT NULL,
 saved_order_id UUID NOT NULL,
 response JSONB NOT NULL,
 PRIMARY KEY(outlet_id,request_id),
 FOREIGN KEY(saved_order_id,outlet_id) REFERENCES saved_orders(id,outlet_id)
);
