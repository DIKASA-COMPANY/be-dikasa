---
name: go-tdd-api-docs
description: Enforce test-driven development and complete documentation for Go backend changes. Use when adding, changing, or fixing Go code, HTTP APIs, database migrations, handlers, services, repositories, or domain logic; require tests before implementation and keep OpenAPI, Bruno, function, and Draw.io ERD documentation current.
---

# Go TDD, API Docs, Bruno & ERD

Use this workflow for every in-scope Go backend change.

## Required order

1. Inspect the affected package, existing tests, migrations, API documentation, `api/bruno`, and `docs/architecture/DIKASA.drawio`.
2. State the behavior and edge cases to cover.
3. Write or update the smallest focused unit test first. Run it and confirm it fails for the expected reason. For a regression, make it reproduce the bug.
4. Implement only enough production code to pass the test.
5. Refactor while keeping tests green.
6. Run the relevant package tests, then `go test ./...` when practical. Do not claim completion if required tests were not run; report the reason.
7. Update OpenAPI, Bruno requests, documentation, and ERD in the same change.

Never write production implementation before its corresponding failing test, except for unavoidable test scaffolding, generated code, or a user-approved exception. Add characterization tests before changing untested existing behavior.

## Unit-test standards

- Keep tests deterministic and independent; do not call real external services, clocks, databases, or networks in unit tests.
- Inject or fake dependencies at package boundaries. Test public behavior, returned errors, authorization outcomes, and important edge cases.
- Use table-driven tests when several inputs express the same rule.
- Test successful and expected failure paths, including invalid input and permission cases for APIs.

## Documentation standards

### Go code

- Add a Go doc comment for every exported package, type, const, var, function, and method added or changed. Begin the comment with the declared identifier.
- Document each new unexported function containing non-obvious business rules, security logic, side effects, or constraints. Explain intent and contract, not line-by-line mechanics.
- Keep comments accurate after refactors; remove stale comments.

### HTTP APIs

Use `/dikasa/v1` as the mandatory base path for every public HTTP API. For example: `POST /dikasa/v1/login`, `GET /dikasa/v1/products`, and `GET /dikasa/v1/transactions/{transaction_id}`. Do not introduce unversioned or differently prefixed public routes without explicit user approval.

For every added or changed endpoint, update the repository's source-of-truth API document (prefer existing OpenAPI/Swagger; otherwise create or update `api/openapi.yaml`). Document method, path, auth/role requirements, parameters, request schema and validation, success schema/status, and meaningful error responses.

Never expose secrets, access tokens, password hashes, or internal error details in examples or documentation.

### Bruno collection

Treat `api/bruno` as an executable API example collection that must stay synchronized with the HTTP contract.

- Add or update at least one `.bru` request for every added or changed endpoint.
- Keep method, `/dikasa/v1` path, authentication mode, headers, path/query parameters, and JSON payload aligned with OpenAPI and the handler DTO.
- Add assertions for the expected success status and important stable response fields. For stateful flows, assign sequence numbers that reflect the usable request order.
- Use environment variables for reusable hosts, credentials, and tokens. Never commit real secrets, access tokens, passwords, or production personal data.
- If an API change does not require a Bruno change, inspect the collection and explicitly report why no update is needed.

### Draw.io ERD

Treat `docs/architecture/DIKASA.drawio` as a required version-controlled architecture artifact.

- Update it in the same change for every database migration or change to a table, column, type, enum/status, primary key, foreign key, index that affects the data model, cardinality, or database constraint.
- Reflect the actual migration/schema: table and column names, nullability, key relationships, and relevant constraints. Do not leave planned entities in the diagram as if they exist.
- For an API-only change with no data-model impact, inspect the ERD and explicitly report that no update is needed.
- Keep the `.drawio` source file, not only a PNG export, in Git. Export a PNG only when the repository already tracks one or the user requests it.
- If the `.drawio` file cannot be edited in the current environment, do not silently skip it: report the blocker and provide the exact required ERD changes.

## Completion checklist

- [ ] Test was written before implementation and observed failing for the intended reason.
- [ ] Focused tests pass; broader Go test command was run or its limitation is reported.
- [ ] New or changed exported identifiers have valid Go doc comments.
- [ ] Non-obvious new functions have intent/contract comments.
- [ ] API documentation reflects each changed endpoint and its errors.
- [ ] Bruno contains a synchronized request and useful assertions for each changed endpoint, or an explicit no-change decision is reported.
- [ ] Every public API route uses the `/dikasa/v1` base path.
- [ ] `docs/architecture/DIKASA.drawio` was updated for data-model changes, or its explicit no-change decision is reported.
- [ ] Final handoff names tests, documentation, and ERD changes.
