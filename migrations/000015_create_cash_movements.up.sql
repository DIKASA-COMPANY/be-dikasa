CREATE TABLE cash_movements (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), outlet_id uuid NOT NULL,
 cash_session_id uuid NOT NULL, created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 created_by_name varchar(201) NOT NULL, created_at timestamptz NOT NULL DEFAULT NOW(),
 request_id uuid NOT NULL,payload_hash char(64) NOT NULL,
 kind varchar(3) NOT NULL CHECK(kind IN ('in','out')),
 amount bigint NOT NULL CHECK(amount BETWEEN 1 AND 9007199254740991),
 name varchar(120) NOT NULL CHECK(length(trim(name))>0),
 reason varchar(500) NOT NULL CHECK(length(trim(reason))>0),
 reverses_id uuid UNIQUE,proof_object_key text,proof_content_type varchar(10),proof_sha256 char(64),proof_size bigint,
 UNIQUE(outlet_id,request_id), UNIQUE(id,cash_session_id,outlet_id),
 FOREIGN KEY(cash_session_id,outlet_id) REFERENCES cash_sessions(id,outlet_id) ON DELETE RESTRICT,
 FOREIGN KEY(reverses_id,cash_session_id,outlet_id) REFERENCES cash_movements(id,cash_session_id,outlet_id) ON DELETE RESTRICT,
 CHECK(reverses_id IS NULL OR reverses_id<>id),
 CHECK((proof_object_key IS NULL AND proof_content_type IS NULL AND proof_sha256 IS NULL AND proof_size IS NULL) OR (proof_object_key IS NOT NULL AND proof_content_type IS NOT NULL AND proof_content_type IN ('image/png','image/jpeg') AND proof_sha256 IS NOT NULL AND proof_size IS NOT NULL AND proof_size BETWEEN 1 AND 3145728))
);
CREATE INDEX cash_movements_session_time ON cash_movements(outlet_id,cash_session_id,created_at,id);
