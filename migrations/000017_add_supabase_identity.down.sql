DROP INDEX IF EXISTS users_supabase_user_id_uidx;
ALTER TABLE users DROP COLUMN IF EXISTS supabase_user_id;
