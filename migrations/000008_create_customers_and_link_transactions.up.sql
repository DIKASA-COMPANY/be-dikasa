CREATE TABLE customers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outlet_id UUID NOT NULL,
    gender_id UUID,
    name VARCHAR(100) NOT NULL,
    address TEXT NOT NULL DEFAULT '',
    points BIGINT NOT NULL DEFAULT 0 CHECK (points >= 0),
    phone_number VARCHAR(30) NOT NULL,
    email VARCHAR(254),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT customers_outlet_fk FOREIGN KEY (outlet_id)
        REFERENCES outlets (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT customers_gender_fk FOREIGN KEY (gender_id)
        REFERENCES genders (id) ON UPDATE CASCADE ON DELETE SET NULL,
    CONSTRAINT customers_id_outlet_unique UNIQUE (id, outlet_id)
);

CREATE UNIQUE INDEX customers_active_phone_uidx
    ON customers (outlet_id, phone_number) WHERE is_active = TRUE;
CREATE UNIQUE INDEX customers_active_email_lower_uidx
    ON customers (outlet_id, LOWER(email)) WHERE is_active = TRUE AND email IS NOT NULL;
CREATE INDEX customers_outlet_name_lower_idx ON customers (outlet_id, LOWER(name));
CREATE INDEX customers_gender_id_idx ON customers (gender_id) WHERE gender_id IS NOT NULL;

ALTER TABLE transactions
    ADD COLUMN customer_id UUID,
    ADD COLUMN customer_name VARCHAR(100) NOT NULL DEFAULT 'Pelanggan Umum',
    ADD CONSTRAINT transactions_customer_outlet_fk FOREIGN KEY (customer_id, outlet_id)
        REFERENCES customers (id, outlet_id) ON UPDATE CASCADE ON DELETE RESTRICT;

CREATE INDEX transactions_customer_id_idx ON transactions (customer_id) WHERE customer_id IS NOT NULL;
