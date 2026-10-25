ALTER TABLE cash_sessions
 ADD COLUMN closed_by UUID REFERENCES users(id),
 ADD COLUMN closed_by_name TEXT,
 ADD COLUMN closing_reason VARCHAR(500),
 ADD COLUMN expected_cash BIGINT CHECK(expected_cash BETWEEN -9007199254740991 AND 9007199254740991),
 ADD COLUMN cash_difference BIGINT CHECK(cash_difference BETWEEN -9007199254740991 AND 9007199254740991),
 ADD COLUMN close_request_id UUID,
 ADD COLUMN close_payload_hash CHAR(64),
 ADD COLUMN close_preview_hash CHAR(64),
 ADD COLUMN closing_summary JSONB,
 ADD CONSTRAINT cash_sessions_reconciliation_check CHECK(
 (closed_by IS NULL AND closed_by_name IS NULL AND closing_reason IS NULL AND expected_cash IS NULL AND cash_difference IS NULL AND close_request_id IS NULL AND close_payload_hash IS NULL AND close_preview_hash IS NULL AND closing_summary IS NULL)
 OR (status='closed' AND closed_by IS NOT NULL AND closed_by_name IS NOT NULL AND closing_reason IS NOT NULL AND expected_cash IS NOT NULL AND cash_difference IS NOT NULL AND close_request_id IS NOT NULL AND close_payload_hash IS NOT NULL AND close_preview_hash IS NOT NULL AND closing_summary IS NOT NULL AND closing_cash::numeric-expected_cash::numeric=cash_difference::numeric AND (cash_difference=0 OR length(trim(closing_reason))>0))
 );
CREATE UNIQUE INDEX cash_sessions_close_request_uidx ON cash_sessions(outlet_id,close_request_id) WHERE close_request_id IS NOT NULL;
