DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM products WHERE image_url IS NOT NULL
        AND image_url NOT LIKE 'https://%.supabase.co/storage/v1/object/public/%'
    ) THEN
        RAISE EXCEPTION 'Migrate existing product images to Supabase Storage public URLs before migration 000016';
    END IF;
    IF EXISTS (
        SELECT 1 FROM users WHERE avatar_url IS NOT NULL
        AND avatar_url NOT LIKE 'https://%.supabase.co/storage/v1/object/public/%'
    ) THEN
        RAISE EXCEPTION 'Migrate existing avatars to Supabase Storage public URLs before migration 000016';
    END IF;
    IF EXISTS (SELECT 1 FROM cash_movements WHERE proof_object_key IS NOT NULL) THEN
        RAISE EXCEPTION 'Migrate existing cash proof objects to Supabase Storage public URLs before migration 000016';
    END IF;
END $$;

ALTER TABLE products DROP COLUMN image_object_name;
ALTER TABLE users DROP COLUMN avatar_object_name;

ALTER TABLE cash_movements
    ADD COLUMN proof_url TEXT,
    DROP COLUMN proof_object_key CASCADE,
    DROP COLUMN proof_content_type CASCADE,
    DROP COLUMN proof_sha256 CASCADE,
    DROP COLUMN proof_size CASCADE;
