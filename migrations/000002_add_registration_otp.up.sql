ALTER TABLE outlets ADD COLUMN email VARCHAR(254);
CREATE UNIQUE INDEX outlets_email_lower_uidx ON outlets (LOWER(email)) WHERE email IS NOT NULL;

CREATE TABLE registration_otps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL UNIQUE,
    outlet_id UUID NOT NULL,
    email VARCHAR(254) NOT NULL,
    code_hash VARCHAR(64) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    resend_available_at TIMESTAMPTZ NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    max_attempts INTEGER NOT NULL DEFAULT 5 CHECK (max_attempts > 0),
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT registration_otps_user_fk FOREIGN KEY (user_id) REFERENCES users (id) ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT registration_otps_outlet_fk FOREIGN KEY (outlet_id) REFERENCES outlets (id) ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE UNIQUE INDEX registration_otps_email_lower_uidx ON registration_otps (LOWER(email));
CREATE INDEX registration_otps_outlet_id_idx ON registration_otps (outlet_id);
CREATE INDEX registration_otps_expires_at_idx ON registration_otps (expires_at) WHERE verified_at IS NULL;
