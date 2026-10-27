ALTER TABLE cash_sessions
    ADD CONSTRAINT cash_sessions_id_outlet_unique UNIQUE (id, outlet_id);

ALTER TABLE order_types
    ADD CONSTRAINT order_types_id_outlet_unique UNIQUE (id, outlet_id);

ALTER TABLE product_variants
    ADD CONSTRAINT product_variants_id_product_outlet_unique UNIQUE (id, product_id, outlet_id);

CREATE TABLE payment_wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outlet_id UUID NOT NULL,
    code VARCHAR(50) NOT NULL,
    name VARCHAR(100) NOT NULL,
    icon_key VARCHAR(100),
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT payment_wallets_outlet_fk FOREIGN KEY (outlet_id)
        REFERENCES outlets (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT payment_wallets_id_outlet_unique UNIQUE (id, outlet_id)
);

CREATE UNIQUE INDEX payment_wallets_active_code_uidx
    ON payment_wallets (outlet_id, LOWER(code)) WHERE is_active = TRUE;
CREATE UNIQUE INDEX payment_wallets_active_name_uidx
    ON payment_wallets (outlet_id, LOWER(name)) WHERE is_active = TRUE;
CREATE INDEX payment_wallets_outlet_sort_idx
    ON payment_wallets (outlet_id, sort_order, name);

INSERT INTO payment_wallets (outlet_id, code, name, icon_key, sort_order)
SELECT id, defaults.code, defaults.name, defaults.icon_key, defaults.sort_order
FROM outlets
CROSS JOIN (VALUES
    ('gopay', 'GoPay', 'gopay', 10),
    ('ovo', 'OVO', 'ovo', 20),
    ('shopeepay', 'ShopeePay', 'shopeepay', 30),
    ('dana', 'DANA', 'dana', 40)
) AS defaults(code, name, icon_key, sort_order);

CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_number VARCHAR(40) NOT NULL UNIQUE,
    outlet_id UUID NOT NULL,
    cash_session_id UUID NOT NULL,
    created_by UUID NOT NULL,
    order_type_id UUID NOT NULL,
    restaurant_table_id UUID,
    payment_method VARCHAR(20) NOT NULL,
    wallet_id UUID,
    subtotal BIGINT NOT NULL CHECK (subtotal >= 0),
    discount_amount BIGINT NOT NULL DEFAULT 0 CHECK (discount_amount >= 0 AND discount_amount <= subtotal),
    tax_percentage NUMERIC(5, 2) NOT NULL CHECK (tax_percentage BETWEEN 0 AND 100),
    tax_amount BIGINT NOT NULL DEFAULT 0 CHECK (tax_amount >= 0),
    total_amount BIGINT NOT NULL CHECK (total_amount >= 0),
    paid_amount BIGINT NOT NULL CONSTRAINT transactions_paid_amount_nonnegative_check CHECK (paid_amount >= 0),
    change_amount BIGINT NOT NULL DEFAULT 0 CHECK (change_amount >= 0),
    rounding_amount BIGINT NOT NULL DEFAULT 0 CHECK (rounding_amount >= 0),
    tip_amount BIGINT NOT NULL DEFAULT 0 CHECK (tip_amount >= 0),
    total_quantity INTEGER NOT NULL CHECK (total_quantity > 0),
    status VARCHAR(20) NOT NULL DEFAULT 'paid' CHECK (status IN ('paid', 'refunded', 'cancelled')),
    paid_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT transactions_payment_method_check CHECK (payment_method IN ('cash', 'wallet', 'debit_card', 'qris', 'point', 'transfer')),
    CONSTRAINT transactions_wallet_method_check CHECK (
        (payment_method = 'wallet' AND wallet_id IS NOT NULL)
        OR (payment_method <> 'wallet' AND wallet_id IS NULL)
    ),
    CONSTRAINT transactions_paid_amount_check CHECK (paid_amount >= total_amount),
    CONSTRAINT transactions_outlet_fk FOREIGN KEY (outlet_id)
        REFERENCES outlets (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT transactions_cash_session_outlet_fk FOREIGN KEY (cash_session_id, outlet_id)
        REFERENCES cash_sessions (id, outlet_id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT transactions_created_by_fk FOREIGN KEY (created_by)
        REFERENCES users (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT transactions_order_type_outlet_fk FOREIGN KEY (order_type_id, outlet_id)
        REFERENCES order_types (id, outlet_id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT transactions_table_outlet_fk FOREIGN KEY (restaurant_table_id, outlet_id)
        REFERENCES restaurant_tables (id, outlet_id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT transactions_wallet_outlet_fk FOREIGN KEY (wallet_id, outlet_id)
        REFERENCES payment_wallets (id, outlet_id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT transactions_id_outlet_unique UNIQUE (id, outlet_id)
);

CREATE INDEX transactions_outlet_paid_at_idx ON transactions (outlet_id, paid_at DESC);
CREATE INDEX transactions_cash_session_id_idx ON transactions (cash_session_id);
CREATE INDEX transactions_created_by_idx ON transactions (created_by);
CREATE INDEX transactions_wallet_id_idx ON transactions (wallet_id) WHERE wallet_id IS NOT NULL;

CREATE TABLE transaction_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID NOT NULL,
    outlet_id UUID NOT NULL,
    product_id UUID NOT NULL,
    product_variant_id UUID,
    product_name VARCHAR(150) NOT NULL,
    variant_name VARCHAR(100),
    unit_price BIGINT NOT NULL CHECK (unit_price >= 0),
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    notes TEXT NOT NULL DEFAULT '',
    line_total BIGINT NOT NULL CHECK (line_total >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT transaction_items_transaction_outlet_fk FOREIGN KEY (transaction_id, outlet_id)
        REFERENCES transactions (id, outlet_id) ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT transaction_items_product_outlet_fk FOREIGN KEY (product_id, outlet_id)
        REFERENCES products (id, outlet_id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT transaction_items_variant_product_outlet_fk FOREIGN KEY (product_variant_id, product_id, outlet_id)
        REFERENCES product_variants (id, product_id, outlet_id) ON UPDATE CASCADE ON DELETE RESTRICT
);

CREATE INDEX transaction_items_transaction_id_idx ON transaction_items (transaction_id);
CREATE INDEX transaction_items_product_id_idx ON transaction_items (product_id);

