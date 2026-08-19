ALTER TABLE products
    ADD COLUMN stock_status VARCHAR(20) NOT NULL DEFAULT 'available',
    ADD COLUMN online_selling_price BIGINT,
    ADD COLUMN cost_price BIGINT,
    ADD COLUMN online_cost_price BIGINT,
    ADD CONSTRAINT products_stock_status_check CHECK (stock_status IN ('available', 'out_of_stock')),
    ADD CONSTRAINT products_online_selling_price_check CHECK (online_selling_price IS NULL OR online_selling_price >= 0),
    ADD CONSTRAINT products_cost_price_check CHECK (cost_price IS NULL OR cost_price >= 0),
    ADD CONSTRAINT products_online_cost_price_check CHECK (online_cost_price IS NULL OR online_cost_price >= 0),
    ADD CONSTRAINT products_id_outlet_unique UNIQUE (id, outlet_id);

CREATE TABLE product_variants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL,
    outlet_id UUID NOT NULL,
    name VARCHAR(100) NOT NULL,
    price BIGINT NOT NULL CHECK (price >= 0),
    online_selling_price BIGINT CHECK (online_selling_price IS NULL OR online_selling_price >= 0),
    cost_price BIGINT CHECK (cost_price IS NULL OR cost_price >= 0),
    online_cost_price BIGINT CHECK (online_cost_price IS NULL OR online_cost_price >= 0),
    stock_status VARCHAR(20) NOT NULL DEFAULT 'available' CHECK (stock_status IN ('available', 'out_of_stock')),
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT product_variants_product_outlet_fk FOREIGN KEY (product_id, outlet_id)
        REFERENCES products (id, outlet_id) ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE INDEX product_variants_product_id_idx ON product_variants (product_id);
CREATE INDEX product_variants_outlet_id_idx ON product_variants (outlet_id);
CREATE UNIQUE INDEX product_variants_name_active_uidx ON product_variants (product_id, LOWER(name)) WHERE is_active = TRUE;

CREATE TABLE reservation_types (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(40) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE restaurant_tables (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outlet_id UUID NOT NULL,
    name VARCHAR(100) NOT NULL,
    capacity INTEGER NOT NULL DEFAULT 1 CHECK (capacity > 0),
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT restaurant_tables_outlet_fk FOREIGN KEY (outlet_id) REFERENCES outlets (id) ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT restaurant_tables_id_outlet_unique UNIQUE (id, outlet_id)
);

CREATE UNIQUE INDEX restaurant_tables_name_lower_uidx ON restaurant_tables (outlet_id, LOWER(name));
CREATE INDEX restaurant_tables_outlet_id_idx ON restaurant_tables (outlet_id);

CREATE TABLE reservations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outlet_id UUID NOT NULL,
    reservation_type_id UUID NOT NULL,
    restaurant_table_id UUID NOT NULL,
    created_by UUID NOT NULL,
    customer_name VARCHAR(100) NOT NULL,
    customer_contact VARCHAR(30) NOT NULL,
    reserved_at TIMESTAMPTZ NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    print_reservation BOOLEAN NOT NULL DEFAULT FALSE,
    send_reservation BOOLEAN NOT NULL DEFAULT FALSE,
    status VARCHAR(20) NOT NULL DEFAULT 'booked' CHECK (status IN ('booked', 'confirmed', 'seated', 'completed', 'cancelled')),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT reservations_outlet_fk FOREIGN KEY (outlet_id) REFERENCES outlets (id) ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT reservations_type_fk FOREIGN KEY (reservation_type_id) REFERENCES reservation_types (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT reservations_table_outlet_fk FOREIGN KEY (restaurant_table_id, outlet_id)
        REFERENCES restaurant_tables (id, outlet_id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT reservations_created_by_fk FOREIGN KEY (created_by) REFERENCES users (id) ON UPDATE CASCADE ON DELETE RESTRICT
);

CREATE UNIQUE INDEX reservations_table_time_uidx ON reservations (outlet_id, restaurant_table_id, reserved_at)
    WHERE is_active = TRUE AND status <> 'cancelled';
CREATE INDEX reservations_outlet_reserved_at_idx ON reservations (outlet_id, reserved_at);
CREATE INDEX reservations_type_id_idx ON reservations (reservation_type_id);
CREATE INDEX reservations_created_by_idx ON reservations (created_by);

INSERT INTO reservation_types (code, name, sort_order) VALUES
    ('regular', 'Reguler', 10),
    ('event', 'Acara', 20),
    ('birthday', 'Ulang Tahun', 30);
