CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE genders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(30) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE outlets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(120) NOT NULL,
    business_type VARCHAR(60) NOT NULL,
    address TEXT NOT NULL,
    phone_number VARCHAR(30) NOT NULL,
    tax_percentage NUMERIC(5, 2) NOT NULL DEFAULT 0 CHECK (tax_percentage BETWEEN 0 AND 100),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id UUID NOT NULL,
    gender_id UUID,
    outlet_id UUID,
    username VARCHAR(50) NOT NULL,
    phone_number VARCHAR(30) NOT NULL UNIQUE,
    email VARCHAR(254),
    password_hash VARCHAR(255) NOT NULL,
    pin_hash VARCHAR(255),
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    date_of_birth DATE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT users_role_fk FOREIGN KEY (role_id) REFERENCES roles (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT users_gender_fk FOREIGN KEY (gender_id) REFERENCES genders (id) ON UPDATE CASCADE ON DELETE SET NULL,
    CONSTRAINT users_outlet_fk FOREIGN KEY (outlet_id) REFERENCES outlets (id) ON UPDATE CASCADE ON DELETE SET NULL
);

CREATE UNIQUE INDEX users_username_lower_uidx ON users (LOWER(username));
CREATE UNIQUE INDEX users_email_lower_uidx ON users (LOWER(email)) WHERE email IS NOT NULL;
CREATE INDEX users_role_id_idx ON users (role_id);
CREATE INDEX users_gender_id_idx ON users (gender_id);
CREATE INDEX users_outlet_id_idx ON users (outlet_id);

INSERT INTO roles (name) VALUES ('admin'), ('manager'), ('cashier');
INSERT INTO genders (name) VALUES ('male'), ('female');
