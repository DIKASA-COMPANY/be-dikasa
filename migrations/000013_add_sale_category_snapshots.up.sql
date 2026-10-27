ALTER TABLE transaction_items
 ADD COLUMN category_id_snapshot UUID,
 ADD COLUMN category_name_snapshot TEXT,
 ADD CONSTRAINT transaction_items_category_snapshot_check CHECK(
 (category_id_snapshot IS NULL AND category_name_snapshot IS NULL)
 OR (category_id_snapshot IS NOT NULL AND category_name_snapshot IS NOT NULL AND length(trim(category_name_snapshot))>0));
