DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM products
        WHERE image_url IS NOT NULL
          AND image_url NOT LIKE 'https://%.supabase.co/storage/v1/object/public/%'
    ) THEN
        RAISE EXCEPTION 'Re-upload every product image to Supabase Storage before migration 000017';
    END IF;
    IF EXISTS (
        SELECT 1 FROM users
        WHERE avatar_url IS NOT NULL
          AND avatar_url NOT LIKE 'https://%.supabase.co/storage/v1/object/public/%'
    ) THEN
        RAISE EXCEPTION 'Re-upload every avatar to Supabase Storage before migration 000017';
    END IF;
    IF EXISTS (
        SELECT 1 FROM cash_movements
        WHERE proof_url IS NOT NULL
          AND proof_url NOT LIKE 'https://%.supabase.co/storage/v1/object/public/%'
    ) THEN
        RAISE EXCEPTION 'Re-upload every cash proof to Supabase Storage before migration 000017';
    END IF;
END $$;

ALTER TABLE users ADD COLUMN supabase_user_id UUID;
CREATE UNIQUE INDEX users_supabase_user_id_uidx
    ON users (supabase_user_id)
    WHERE supabase_user_id IS NOT NULL;
