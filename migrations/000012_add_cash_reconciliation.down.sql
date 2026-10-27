DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM cash_sessions WHERE close_request_id IS NOT NULL) THEN
 RAISE EXCEPTION 'Refusing to erase cash reconciliation history'; END IF;
END $$;
DROP INDEX cash_sessions_close_request_uidx;
ALTER TABLE cash_sessions DROP CONSTRAINT cash_sessions_reconciliation_check,
 DROP COLUMN closed_by, DROP COLUMN closed_by_name, DROP COLUMN closing_reason,
 DROP COLUMN expected_cash, DROP COLUMN cash_difference, DROP COLUMN close_request_id,
 DROP COLUMN close_payload_hash, DROP COLUMN close_preview_hash, DROP COLUMN closing_summary;
