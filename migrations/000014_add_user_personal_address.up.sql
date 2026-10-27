ALTER TABLE users ADD COLUMN personal_address text;
ALTER TABLE users ADD CONSTRAINT users_personal_address_length CHECK (personal_address IS NULL OR char_length(personal_address) <= 1000);
