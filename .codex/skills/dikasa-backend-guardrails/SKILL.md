---
name: dikasa-backend-guardrails
description: Build, change, or review the DIKASA Go backend while preserving its feature-based architecture, PostgreSQL persistence, Firebase-only media and notifications, Huma APIs, go-playground validation, tests, and synchronized API/architecture documentation. Use only for the be-dikasa backend repository.
---

# DIKASA Backend Guardrails

Apply these invariants to every backend implementation and review in this repository.

## Architecture

- Organize business code by feature under `internal/<feature>`. Keep handlers, DTOs, services, repositories, entities, and feature tests close to the owning feature.
- Keep HTTP handlers thin: parse the Huma DTO, obtain authenticated identity, invoke the service, and translate domain errors. Put business rules in services and persistence details in repositories.
- Put genuinely reusable infrastructure under `internal/shared`; do not create cross-feature dependencies for convenience.
- Use `/dikasa/v1` for public API routes. Define and register APIs through Huma so generated OpenAPI and Swagger remain authoritative.
- Use `go-playground/validator` through the repository's shared validator for request rules that cannot be expressed completely by Huma tags. Do not add ad-hoc competing validation frameworks.

## Required infrastructure

- PostgreSQL is the primary database for identities, business data, and external-resource references. Read its DSN, including username and password, from `DATABASE_URL`; never hardcode credentials or commit secrets.
- Use migrations for every schema change. Keep GORM models and SQL constraints aligned. PostgreSQL stores image URLs only, never image bytes or duplicate Firebase object metadata.
- Use Firebase Authentication for runtime tokens and identity verification. Do not add a local-auth runtime mode.
- Send application notifications only through Firebase Cloud Messaging. Initialize it from Firebase project credentials supplied through environment configuration/ADC, normally `FIREBASE_PROJECT_ID` and `GOOGLE_APPLICATION_CREDENTIALS`. Never hardcode or commit a Firebase server key. Add startup validation and `.env.example` documentation when new notification configuration is required.
- Store every uploaded image only in Firebase Storage through the shared uploader. Read the bucket from `FIREBASE_STORAGE_BUCKET` and credentials from environment configuration/ADC. Validate the file with the shared image policy before upload, persist only the resulting Firebase URL in PostgreSQL, and return that URL for clients to render rather than proxying file bytes.
- Keep object names tenant-scoped and collision-safe. Follow `outlets/{outlet_id}/<feature>/<resource_id>/<purpose>/{uuid}.{ext}` unless an established feature prefix is more specific.

## Test-first workflow

1. Inspect the feature, tests, migration history, OpenAPI source/generator, Bruno collection, and ERD before editing.
2. Add the smallest deterministic unit test first and run it to observe the intended failure.
3. Implement only enough code to satisfy the behavior, then refactor while tests stay green.
4. Fake Firebase, clocks, and repositories in unit tests; never call real Firebase, PostgreSQL, or notification services from unit tests.
5. Cover success, invalid input, authorization/tenant isolation, storage/provider failure, and cleanup or transaction rollback where relevant.
6. Run focused tests, `go generate ./internal/server`, `go test ./...`, `go vet ./...`, and `git diff --check`. Report any opt-in PostgreSQL integration test that could not run.

## Documentation contract

For every changed HTTP endpoint or model:

- Regenerate `api/openapi.yaml` and verify Swagger reflects the same contract.
- Add or update the matching request in `api/bruno`, using environment variables and stable assertions without real secrets.
- Update relevant README/development documentation, including configuration, authorization, failure behavior, and client usage.
- Update `docs/architecture/DIKASA.drawio` whenever a migration or persisted relationship changes. For an API-only change, inspect the ERD and explicitly record that no ERD change was required.
- Keep exported Go identifiers documented and remove stale comments or documentation for deleted behavior.

Do not mark work complete while implementation, tests, OpenAPI/Swagger, Bruno, ERD, or relevant documentation disagree.
