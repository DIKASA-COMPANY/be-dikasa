DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM transaction_items WHERE category_id_snapshot IS NOT NULL) THEN
 RAISE EXCEPTION 'Refusing to erase original sale categories'; END IF;
END $$;
ALTER TABLE transaction_items DROP CONSTRAINT transaction_items_category_snapshot_check,
 DROP COLUMN category_id_snapshot, DROP COLUMN category_name_snapshot;
