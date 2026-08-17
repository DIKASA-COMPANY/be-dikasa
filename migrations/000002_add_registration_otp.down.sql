DROP TABLE IF EXISTS registration_otps;
DROP INDEX IF EXISTS outlets_email_lower_uidx;
ALTER TABLE outlets DROP COLUMN IF EXISTS email;
