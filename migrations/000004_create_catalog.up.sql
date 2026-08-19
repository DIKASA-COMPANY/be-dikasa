CREATE TABLE product_types (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(30) NOT NULL UNIQUE,
    name VARCHAR(60) NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE product_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outlet_id UUID NOT NULL,
    product_type_id UUID NOT NULL,
    name VARCHAR(100) NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT product_categories_outlet_fk FOREIGN KEY (outlet_id) REFERENCES outlets (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT product_categories_product_type_fk FOREIGN KEY (product_type_id) REFERENCES product_types (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT product_categories_id_outlet_unique UNIQUE (id, outlet_id)
);
CREATE UNIQUE INDEX product_categories_active_name_uidx
    ON product_categories (outlet_id, product_type_id, LOWER(name)) WHERE is_active;
CREATE INDEX product_categories_outlet_type_idx ON product_categories (outlet_id, product_type_id);

CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outlet_id UUID NOT NULL,
    category_id UUID NOT NULL,
    name VARCHAR(150) NOT NULL,
    price BIGINT NOT NULL CHECK (price >= 0),
    description TEXT NOT NULL DEFAULT '',
    image_url TEXT,
    image_object_name TEXT,
    is_highlighted BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT products_outlet_fk FOREIGN KEY (outlet_id) REFERENCES outlets (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT products_category_outlet_fk FOREIGN KEY (category_id, outlet_id) REFERENCES product_categories (id, outlet_id) ON UPDATE CASCADE ON DELETE RESTRICT
);
CREATE INDEX products_outlet_category_idx ON products (outlet_id, category_id);
CREATE INDEX products_outlet_name_lower_idx ON products (outlet_id, LOWER(name));

CREATE TABLE order_types (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outlet_id UUID NOT NULL,
    code VARCHAR(50) NOT NULL,
    name VARCHAR(100) NOT NULL,
    icon_key VARCHAR(100),
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT order_types_outlet_fk FOREIGN KEY (outlet_id) REFERENCES outlets (id) ON UPDATE CASCADE ON DELETE RESTRICT
);
CREATE UNIQUE INDEX order_types_active_code_uidx ON order_types (outlet_id, LOWER(code)) WHERE is_active;
CREATE UNIQUE INDEX order_types_active_name_uidx ON order_types (outlet_id, LOWER(name)) WHERE is_active;

INSERT INTO product_types (code, name, sort_order) VALUES
    ('food', 'Makanan', 10),
    ('drink', 'Minuman', 20),
    ('additional', 'Tambahan', 30);

INSERT INTO order_types (outlet_id, code, name, sort_order)
SELECT id, defaults.code, defaults.name, defaults.sort_order
FROM outlets
CROSS JOIN (VALUES
    ('free_table', 'Bebas Pilih Meja', 10),
    ('table_order', 'Pesan Meja', 20),
    ('takeaway', 'Bungkus', 30),
    ('delivery', 'Pesan Antar', 40),
    ('reservation', 'Reservasi', 50),
    ('online_delivery', 'Ojek Online', 60)
) AS defaults(code, name, sort_order);

UPDATE users AS u
SET role_id = m.id, updated_at = NOW()
FROM roles AS r, roles AS m
WHERE u.role_id = r.id
  AND u.outlet_id IS NOT NULL
  AND LOWER(r.name) = 'admin'
  AND LOWER(m.name) = 'manager';
