DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM cash_movements WHERE proof_url IS NOT NULL) THEN
        RAISE EXCEPTION 'Refusing to discard persisted Firebase proof URLs';
    END IF;
END $$;

ALTER TABLE products ADD COLUMN image_object_name TEXT;
ALTER TABLE users ADD COLUMN avatar_object_name TEXT;

ALTER TABLE cash_movements
    ADD COLUMN proof_object_key TEXT,
    ADD COLUMN proof_content_type VARCHAR(10),
    ADD COLUMN proof_sha256 CHAR(64),
    ADD COLUMN proof_size BIGINT,
    DROP COLUMN proof_url;

ALTER TABLE cash_movements
    ADD CONSTRAINT cash_movements_proof_metadata_check CHECK (
        (proof_object_key IS NULL AND proof_content_type IS NULL AND proof_sha256 IS NULL AND proof_size IS NULL)
        OR
        (proof_object_key IS NOT NULL AND proof_content_type IS NOT NULL
            AND proof_content_type IN ('image/png', 'image/jpeg')
            AND proof_sha256 IS NOT NULL AND proof_size IS NOT NULL
            AND proof_size BETWEEN 1 AND 3145728)
    );
