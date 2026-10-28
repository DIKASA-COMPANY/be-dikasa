.PHONY: run test test-cover vet fmt openapi migrate-up migrate-down

run:
	go run ./cmd/api

test:
	go test ./...

test-cover:
	go test -cover ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

openapi:
	go generate ./internal/server

migrate-up:
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f migrations/000001_create_identity_tables.up.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f migrations/000002_add_registration_otp.up.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f migrations/000003_create_cash_sessions.up.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f migrations/000004_create_catalog.up.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f migrations/000005_add_user_profile_avatar.up.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f migrations/000006_add_reservations_and_product_commerce.up.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f migrations/000007_create_payment_wallets_and_transactions.up.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f migrations/000008_create_customers_and_link_transactions.up.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -1 -f migrations/000009_add_product_stock.up.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -1 -f migrations/000010_add_saved_orders.up.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -1 -f migrations/000011_add_transaction_refunds.up.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -1 -f migrations/000012_add_cash_reconciliation.up.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -1 -f migrations/000013_add_sale_category_snapshots.up.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -1 -f migrations/000014_add_user_personal_address.up.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -1 -f migrations/000015_create_cash_movements.up.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -1 -f migrations/000016_store_only_firebase_image_urls.up.sql

migrate-down:
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -1 -f migrations/000016_store_only_firebase_image_urls.down.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -1 -f migrations/000015_create_cash_movements.down.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -1 -f migrations/000014_add_user_personal_address.down.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -1 -f migrations/000013_add_sale_category_snapshots.down.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -1 -f migrations/000012_add_cash_reconciliation.down.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -1 -f migrations/000011_add_transaction_refunds.down.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -1 -f migrations/000010_add_saved_orders.down.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -1 -f migrations/000009_add_product_stock.down.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f migrations/000008_create_customers_and_link_transactions.down.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f migrations/000007_create_payment_wallets_and_transactions.down.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f migrations/000006_add_reservations_and_product_commerce.down.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f migrations/000005_add_user_profile_avatar.down.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f migrations/000004_create_catalog.down.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f migrations/000003_create_cash_sessions.down.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f migrations/000002_add_registration_otp.down.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f migrations/000001_create_identity_tables.down.sql
