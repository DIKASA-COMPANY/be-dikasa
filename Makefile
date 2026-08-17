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

migrate-down:
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f migrations/000002_add_registration_otp.down.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f migrations/000001_create_identity_tables.down.sql
