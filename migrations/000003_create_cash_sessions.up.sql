CREATE TABLE cash_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outlet_id UUID NOT NULL,
    user_id UUID NOT NULL,
    opening_cash BIGINT NOT NULL CHECK (opening_cash >= 0 AND opening_cash <= 9007199254740991),
    closing_cash BIGINT CHECK (closing_cash >= 0 AND closing_cash <= 9007199254740991),
    status VARCHAR(20) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed')),
    opened_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT cash_sessions_outlet_fk FOREIGN KEY (outlet_id) REFERENCES outlets (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT cash_sessions_user_fk FOREIGN KEY (user_id) REFERENCES users (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT cash_sessions_closing_state_check CHECK (
        (status = 'open' AND closing_cash IS NULL AND closed_at IS NULL)
        OR (status = 'closed' AND closing_cash IS NOT NULL AND closed_at IS NOT NULL)
    )
);

CREATE UNIQUE INDEX cash_sessions_outlet_open_uidx ON cash_sessions (outlet_id) WHERE status = 'open';
CREATE INDEX cash_sessions_user_id_idx ON cash_sessions (user_id);
CREATE INDEX cash_sessions_opened_at_idx ON cash_sessions (opened_at DESC);
