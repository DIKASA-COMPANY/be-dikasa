# Dikasa Backend

Product, avatar, and cash-proof uploads use Supabase Storage signed upload URLs exclusively.
The backend authorizes an intent, the client uploads directly to Supabase, and the backend
validates the stored image before persisting its public URL.
Shared stock tracking, manager inventory adjustments/history, variation-aware
checkout validation and real XLSX exports are available. See
[product images and stock setup](docs/development/product-stock-and-images.md).

Dikasa Backend is a Go API for restaurant operations. The current implementation
provides business-owner registration with email OTP verification, PostgreSQL
credential validation, Supabase Auth sessions and OAuth identity binding, authenticated POS catalogs,
opening-cash sessions, outlet order types, and generated OpenAPI documentation.

## Contents

- [Technology stack](#technology-stack)
- [How authentication works](#how-authentication-works)
- [Project structure](#project-structure)
- [Prerequisites](#prerequisites)
- [First-time setup](#first-time-setup)
- [Running the API](#running-the-api)
- [Using Bruno](#using-bruno)
- [API overview](#api-overview)
- [Development workflow](#development-workflow)
- [Troubleshooting](#troubleshooting)

## Technology stack

- Go 1.26.5
- Gin for HTTP routing and middleware
- Huma for typed HTTP operations and OpenAPI generation
- GORM with PostgreSQL
- Supabase Auth for access/refresh tokens and Google/Facebook OAuth identities
- Supabase Storage for every uploaded product, avatar, and cash-proof image
- SMTP for registration OTP delivery
- `go-playground/validator` for request validation
- Bruno for manual API testing
- Draw.io for the database ERD

## How authentication works

PostgreSQL is the primary database and source of truth for users, business data,
password hashes, Supabase identity bindings, and public image URLs. Supabase provides
Auth and object storage; it does not replace PostgreSQL business persistence.

The login flow is:

1. The API finds the user in PostgreSQL by username, phone number, or email.
2. The submitted password is compared with the PostgreSQL bcrypt hash.
3. The API signs in through Supabase Auth. On the first login, an existing UUID and
   bcrypt hash are imported without requiring a password reset.
4. The API returns the Supabase access token, refresh token, and bound Auth user ID.
5. Protected endpoints verify the access token with Supabase Auth and resolve
   `users.supabase_user_id`. A confirmed Google/Facebook email may bind to an existing
   active DIKASA account with the same unique email.

The registration flow creates an inactive outlet and user. Both are activated
only after the selected email address is verified with an OTP.

## Project structure

```text
be-dikasa/
├── cmd/
│   ├── api/                    # Application entry point and dependency wiring
│   └── openapi/                # OpenAPI generator
├── internal/
│   ├── auth/                   # Login and Supabase account migration
│   ├── cash/                   # Opening-cash sessions
│   ├── catalog/                # Products, categories, order types, image storage
│   ├── config/                 # Environment configuration
│   ├── database/               # PostgreSQL client
│   ├── registration/           # Register, confirm OTP, resend OTP, and SMTP
│   ├── server/                 # Gin/Huma router and authentication middleware
│   ├── shared/                 # Shared errors, responses, and validation
│   └── user/                   # User entity, repository, service, and profile API
├── migrations/                 # PostgreSQL up/down migrations and tests
├── api/
│   ├── openapi.yaml            # Generated API contract
│   └── bruno/                  # Bruno collection and local environment
├── docs/
│   ├── architecture/DIKASA.drawio
│   └── development/naming.md
├── .env.example                # Environment variable reference
└── Makefile                    # Common development commands
```

## Prerequisites

Install or obtain access to:

- Go 1.26.5 or the version declared in `go.mod`.
- PostgreSQL and the `psql` command-line client.
- Permission to enable the PostgreSQL `pgcrypto` extension.
- A Supabase project with email/password Auth enabled and optional Google/Facebook providers.
- A Supabase publishable key and server-only secret key.
- A public Supabase Storage bucket, for example `dikasa-images`.
- An SMTP account that can send OTP emails.
- GNU Make, if you want to use the Makefile commands. Direct alternatives are
  included below.
- Bruno Desktop, if you want to run the included API collection.

## First-time setup

### 1. Clone the repository

```bash
git clone https://github.com/GerhanaAbiW/be-dikasa.git
cd be-dikasa
go mod download
```

### 2. Create a PostgreSQL database

Create a local database and user using your preferred PostgreSQL tool. The
examples in this repository expect a connection string similar to:

```text
postgres://dikasa:password@localhost:5432/dikasa?sslmode=disable
```

Do not use the example password in shared or production environments.

### 3. Prepare Supabase

From the Supabase project used by your team:

1. Copy the Project URL from **Project Settings → API**.
2. Create/copy a publishable key and a server-only secret key. Never expose the secret key to mobile or web clients.
3. Create a public Storage bucket such as `dikasa-images`.
4. Enable email/password Auth. Configure Google and Facebook in **Authentication → Providers** when required.
5. Add the frontend callback URLs to the Supabase Auth redirect allowlist.

Password accounts preserve `users.id` when imported. OAuth accounts can use a different
Supabase UUID, stored in `users.supabase_user_id`.

### 4. Configure email delivery

Obtain the SMTP host, port, username, password, and sender address from the
team's email provider. Registration succeeds only when the API can send the OTP
to the selected email address.

### 5. Set environment variables

Copy `.env.example` to an untracked `.env` for local development, or export the
variables in your terminal. Existing process environment variables always win
over `.env` values.

PowerShell example:

```powershell
$env:PORT = "8080"
$env:DATABASE_URL = "postgres://dikasa:password@localhost:5432/dikasa?sslmode=disable"
$env:SUPABASE_URL = "https://your-project-ref.supabase.co"
$env:SUPABASE_PUBLISHABLE_KEY = "sb_publishable_replace_me"
$env:SUPABASE_SECRET_KEY = "sb_secret_replace_me"
$env:SUPABASE_STORAGE_BUCKET = "dikasa-images"
$env:OTP_SECRET = "replace-with-a-random-secret-of-at-least-32-characters"
$env:SMTP_HOST = "smtp.example.com"
$env:SMTP_PORT = "587"
$env:SMTP_USERNAME = "mailer@example.com"
$env:SMTP_PASSWORD = "your-smtp-password"
$env:SMTP_FROM = "Dikasa <mailer@example.com>"
```

Bash example:

```bash
export PORT='8080'
export DATABASE_URL='postgres://dikasa:password@localhost:5432/dikasa?sslmode=disable'
export SUPABASE_URL='https://your-project-ref.supabase.co'
export SUPABASE_PUBLISHABLE_KEY='sb_publishable_replace_me'
export SUPABASE_SECRET_KEY='sb_secret_replace_me'
export SUPABASE_STORAGE_BUCKET='dikasa-images'
export OTP_SECRET='replace-with-a-random-secret-of-at-least-32-characters'
export SMTP_HOST='smtp.example.com'
export SMTP_PORT='587'
export SMTP_USERNAME='mailer@example.com'
export SMTP_PASSWORD='your-smtp-password'
export SMTP_FROM='Dikasa <mailer@example.com>'
```

`OTP_SECRET` must be at least 32 characters and must be different for each
environment. Never commit real database, Supabase secret, SMTP, or OTP credentials.

### 6. Run database migrations

With Make:

```bash
make migrate-up
```

Without Make:

```bash
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/000001_create_identity_tables.up.sql
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/000002_add_registration_otp.up.sql
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/000003_create_cash_sessions.up.sql
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/000004_create_catalog.up.sql
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/000005_add_user_profile_avatar.up.sql
# Continue in numeric order through:
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -1 -f migrations/000016_store_only_image_urls.up.sql
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -1 -f migrations/000017_add_supabase_identity.up.sql
```

PowerShell users can replace `"$DATABASE_URL"` with `$env:DATABASE_URL`.

The migrations create the `roles`, `genders`, `outlets`, `users`,
`registration_otps`, `cash_sessions`, `product_types`, `product_categories`,
`products`, and `order_types` tables. Passwords are stored as bcrypt
hashes. OTP values are stored as HMAC-SHA256 hashes and are never persisted as
plaintext. Opening cash is stored as a whole-Rupiah integer per outlet cash
session, with at most one open session per outlet.

Migration 000017 refuses to proceed while product, avatar, or cash-proof rows still
contain non-Supabase URLs. Re-upload those objects first so the database cannot retain
stale provider links. After the migration and Supabase environment variables are ready,
import every active email account while preserving UUIDs and bcrypt hashes:

```bash
make migrate-supabase-users
```

The command is safe to resume: already bound accounts are skipped. If an exact-email
Google/Facebook identity already exists, it is linked instead of creating a duplicate.
The `/login` endpoint also performs the same migration lazily as a fallback.

Supabase Storage is mandatory at startup and the configured bucket must be public.
Images are limited to 3 MiB. Declared type/size are checked before signing, then the
stored PNG/JPEG bytes, dimensions, and pixel count are verified during completion.
Product and profile responses return public URLs; frontends render those URLs directly. PINs use six digits, and both
PINs and passwords are persisted only as bcrypt hashes.

## Running the API

With Make:

```bash
make run
```

Without Make:

```bash
go run ./cmd/api
```

The default base URL is `http://localhost:8080/dikasa/v1`.

After the server starts, open:

- Swagger UI: `http://localhost:8080/dikasa/v1/docs`
- Runtime OpenAPI JSON: `http://localhost:8080/dikasa/v1/openapi.json`
- Runtime OpenAPI YAML: `http://localhost:8080/dikasa/v1/openapi.yaml`

## Using Bruno

The repository includes a ready-to-use Bruno collection in `api/bruno`.

### Open the collection

1. Start the API locally.
2. Open Bruno Desktop.
3. Select **Open Collection**.
4. Choose the repository folder `api/bruno`.
5. Select the **local** environment in Bruno.
6. Confirm that `baseUrl` is `http://localhost:8080`.

### First-time registration flow

A new database contains roles and genders but no application user. Run these
requests in order:

1. **Register**
   - Replace the example email addresses and phone numbers with unique values.
   - `verification_email` must exactly match either `owner_email` or
     `business_email`.
   - The response should be HTTP `201` with `otp_expires_in: 300`.
2. **Resend Registration OTP** — optional
   - Use this only if the original email did not arrive.
   - Wait for the 60-second cooldown; an early request returns HTTP `429`.
   - A successful resend invalidates the previous code.
3. **Confirm Registration OTP**
   - Read the real five-digit code from the email inbox.
   - Replace the `12345` placeholder in the Bruno request.
   - The response should be HTTP `200` with `verified: true`.
4. **Login**
   - Set `loginIdentifier` and `loginPassword` in the Bruno **local**
     environment to the username and password used by **Register**.
   - Send the request. Its post-response script automatically stores
     `accessToken` and `refreshToken` in the selected environment.
5. **Health Check** and **My Profile**
   - These requests automatically use `accessToken` as a Bearer token.

Use a new username, owner phone, owner email, and business email when repeating
the flow. PostgreSQL uniqueness constraints intentionally reject duplicate
registrations with HTTP `409`.

The committed `.bru` files contain only local example data. Never save real
production credentials or access tokens in the collection.

## API overview

All application routes use the `/dikasa/v1` prefix.

| Method | Path | Authentication | Purpose |
| --- | --- | --- | --- |
| `POST` | `/register` | Public | Create an inactive outlet/user and send an OTP |
| `POST` | `/register/otp/resend` | Public | Rotate and resend a registration OTP |
| `POST` | `/register/otp/confirm` | Public | Verify the OTP and activate the outlet/user |
| `POST` | `/login` | Public | Validate PostgreSQL credentials and issue Supabase tokens |
| `GET` | `/users/me` | Supabase Bearer token | Return the authenticated PostgreSQL profile |
| `POST` | `/users/me/avatar-upload-intents` | Supabase Bearer token | Create a signed direct-upload URL for the caller's avatar |
| `PATCH` | `/users/me` | Supabase Bearer token | Partially update profile or adopt `avatar_object_name` |
| `PATCH` | `/users/me/pin` | Supabase Bearer token | Set or change the six-digit account PIN |
| `PATCH` | `/users/me/password` | Supabase Bearer token | Verify the current password and set a new password |
| `POST` | `/cash-sessions` | Supabase Bearer token | Open the outlet cash session with an opening-cash amount |
| `GET` | `/cash-sessions/current` | Supabase Bearer token | Return the outlet's active cash session |
| `GET` | `/payments/tax-percentage` | Supabase Bearer token | Return only the current outlet tax percentage for frontend payment calculation |
| `GET/POST` | `/wallets` | User / Manager | List active wallet options or create an outlet wallet |
| `PATCH/DELETE` | `/wallets/{wallet_id}` | Manager | Update or soft-delete an outlet wallet |
| `POST` | `/transactions` | User outlet with active cash session | Atomically save the paid transaction and all product snapshots |
| `GET` | `/orders` | Supabase Bearer token | List outlet orders with receipt/customer search, type/status filters, and pagination |
| `GET` | `/customers` | Supabase Bearer token | List active registered customers with search, gender filter, and pagination |
| `GET` | `/product-types` | Supabase Bearer token | Return Food, Drink, and Additional tabs |
| `GET` | `/products` | Supabase Bearer token | Return outlet products with category, type, search, and pagination filters |
| `GET` | `/products/{product_id}` | Supabase Bearer token | Return one outlet product |
| `POST` | `/products/image-upload-intents` | Manager | Create a signed direct-upload URL for a product image |
| `POST/PATCH/DELETE` | `/products` and `/products/{product_id}` | Manager | Manage products with soft delete |
| `POST` | `/cash-sessions/{session_id}/movement-proof-upload-intents` | Opening cashier / Manager | Create a request-bound signed proof upload URL |
| `GET/POST` | `/product-types/{product_type}/categories` | User / Manager | List or create outlet categories for a product type |
| `PATCH/DELETE` | `/product-types/{product_type}/categories/{category_id}` | Manager | Update or soft-delete a category |
| `GET/POST` | `/order-types` | User / Manager | List or create outlet order types |
| `PATCH/DELETE` | `/order-types/{order_type_id}` | Manager | Update or soft-delete an order type |
| `GET` | `/reservation-types` | Supabase Bearer token | Return the seeded reservation-type dropdown |
| `GET` | `/restaurant-tables` | Supabase Bearer token | Return active outlet tables; optionally filter availability by `reservation_at` |
| `POST/PATCH/DELETE` | `/restaurant-tables` and `/restaurant-tables/{table_id}` | Manager | Configure outlet tables with soft delete |
| `GET/POST` | `/reservations` | User outlet | List or create outlet reservations |
| `GET/PATCH/DELETE` | `/reservations/{reservation_id}` | User outlet | Read, update, or cancel an outlet reservation |
| `GET` | `/health` | Supabase Bearer token | Confirm the authenticated API path is healthy |
| `GET` | `/docs` | Public | Serve Swagger UI |
| `GET` | `/openapi.yaml` | Public | Serve the generated OpenAPI document |

Product writes use `multipart/form-data`. The create form follows the product
screen: `name`, `category_id`, `product_type`, `selling_price`, `stock_status`,
optional `online_selling_price`, `cost_price`, `online_cost_price`, `description`,
and `image_object_name`. Image bytes are uploaded directly to the signed Supabase URL
before the product mutation. The optional `variations` part is `application/json` containing an
array with the same price and stock fields. `price` remains in responses as a
compatibility alias of `selling_price`. “Simpan & Buat Lagi” repeats the same
POST request; spreadsheet export is outside this create-product endpoint.

Reservation creation stores customer/contact, a future RFC 3339
`reserved_at`, reservation type, restaurant table, notes, and the print/send
flags. A table cannot have two active reservations at the exact same timestamp.
The flags are persisted instructions only; no message is sent until a separate
communication provider is configured.

The payment page requests `GET /payments/tax-percentage` when it opens. Its
response is only `{"tax_percentage":10}`, where `10` means 10 percent. The
frontend keeps the order list and calculates tax and totals locally; the API
does not receive or recalculate the cart for this lookup.

Wallets are outlet-scoped dropdown options. New outlets receive GoPay, OVO,
ShopeePay, and DANA by default; managers may add, edit, reorder, deactivate, or
soft-delete them. A wallet payment must send an active `wallet_id`.

When **Proses Bayar** is confirmed, `POST /transactions` accepts the FE totals
and item snapshots. Each item records product/variant IDs, display names, unit
price, quantity, notes, and line total. The backend performs lightweight
arithmetic and outlet validation in Go, requires an open cash session, creates
the transaction number, and commits the header and all items in one database
transaction. It does not run a database aggregation to recalculate the cart.

The order-history screen uses `GET /orders`. Results are isolated to the
authenticated user's outlet and ordered newest first. `search` matches receipt
number or the stored customer-name snapshot; `order_type_id`, `status`, `page`,
and `page_size` are optional filters. The response keeps order type and table as
structured objects so the frontend can render labels such as `Dine-In [Meja 5]`.

The registered-customer screen uses `GET /customers`. `search` matches name,
phone number, or email; `gender_id`, `page`, and `page_size` are optional.
Customers are outlet-scoped and distinct from staff accounts in `users`.

Important OTP rules:

- The OTP contains five digits.
- It expires after five minutes.
- Resend is available after 60 seconds.
- Each code allows at most five failed attempts.
- Resending rotates the hash, invalidates the previous code, and resets attempts.
- The user and outlet remain inactive until successful confirmation.

Error responses use a consistent envelope:

```json
{
  "error": {
    "code": "VALIDASI_GAGAL",
    "message": "Data yang dikirim tidak memenuhi ketentuan.",
    "details": {
      "field_name": "Field-specific message."
    }
  }
}
```

## Development workflow

This project follows test-driven development for Go backend changes.

```bash
make test          # Run all Go tests
make test-cover    # Run tests with package coverage
make vet           # Run Go static checks
make fmt           # Format Go packages
make openapi       # Regenerate api/openapi.yaml
```

Direct commands are also available:

```bash
go test ./...
go vet ./...
go fmt ./...
go generate ./internal/server
```

When adding or changing an endpoint:

1. Add a focused failing test before production code.
2. Implement the smallest change that makes the test pass.
3. Keep public routes under `/dikasa/v1`.
4. Regenerate `api/openapi.yaml`; do not edit generated schemas manually.
5. Add or update the matching request in `api/bruno` with useful assertions.
6. Update `docs/architecture/DIKASA.drawio` when the data model changes.
7. Run `go test ./...` and `go vet ./...`.

Additional project references:

- API artifact notes: `api/README.md`
- Naming conventions: `docs/development/naming.md`
- Database ERD: `docs/architecture/DIKASA.drawio`
- Backend workflow skill: `.codex/skills/go-tdd-api-docs/SKILL.md`

## Rolling back migrations

> Warning: rollback removes the cash-session, registration, and identity tables
> and destroys their data. Use it only in a disposable local database.

```bash
make migrate-down
```

The reservation/product-commerce migration is rolled back before user profile,
catalog, cash sessions, registration, and identity to
preserve foreign-key ordering.

## Troubleshooting

### The API exits with missing configuration

The application loads `.env` from the working directory and validates required
variables during startup. Compare your configuration with `.env.example` or
`.env.local.example`. Existing process variables take precedence over `.env`.

### PostgreSQL migration fails on `pgcrypto`

The database user must be allowed to create or use the `pgcrypto` extension.
Ask the database administrator to enable it when using a managed database.

### Supabase initialization fails

Check `SUPABASE_URL`, `SUPABASE_PUBLISHABLE_KEY`, and `SUPABASE_SECRET_KEY`. Confirm
that `SUPABASE_STORAGE_BUCKET` exists in the same project and is public.

### Login returns a Supabase provider error

Confirm that email/password Auth is enabled and that the server-only secret key can
call the Auth Admin API. Never place the secret key in a client application.

### Registration returns `EMAIL_GAGAL_DIKIRIM`

Check the SMTP host, port, authentication credentials, sender address, and the
provider's STARTTLS requirements. The pending registration remains available;
after the cooldown, use the resend endpoint with the selected verification
email.

### Protected requests return HTTP 401

Run **Login** in Bruno again and make sure the selected environment contains a
non-empty `accessToken`. Supabase access tokens expire and must be refreshed or
issued again.

### Registration returns HTTP 409

At least one unique value is already registered. Use a different username,
owner phone, owner email, or business email for a new local test registration.

## Saved unpaid orders

Migration000010 connects persistent draft carts, outlet browsing, revision-safe editing,
retry replay and atomic draft checkout. See [saved-orders.md](docs/development/saved-orders.md)
for setup, recovery, APIs, Bruno examples and tests.

## Refunds and cancellation

Whole-remaining and item-quantity refunds preserve original receipts and record
manually returned money. Stock return is explicit and requires original tracked-sale
evidence. Migration000011 is required; see [refund setup/API guide](docs/development/refunds.md).

Cash-session closing, reconciliation and recovery: [Cash reconciliation guide](docs/development/cash-reconciliation.md).

Four sales reports and XLSX: [Reporting guide](docs/development/sales-reports.md).


## Account settings (24 October 2026)

The cashier menu's **Pengaturan Akun** supports self-profile details, Supabase profile
photos, PIN setup/change and PostgreSQL password changes. Restart the backend and
Flutter after updating. Migration000014 (nullable personal address) is already
applied in this workspace's local database. Fresh installs must also apply
`migrations/000014_add_user_personal_address.up.sql` in the backend after000013.

Photos are stored under `outlets/{outlet_id}/users/{user_id}/avatar/` in the configured
Supabase bucket. The avatar intent is actor-bound; `PATCH /users/me` adopts only its exact
`avatar_object_name` after server-side content verification. Password changes update the PostgreSQL credential used by `/login`;
the bcrypt hash is synchronized to Supabase Auth on the next password login and does
not revoke access tokens already issued.


## Cash-in/out and till activity (24 October2026)

Restart the backend and Flutter, then choose **Menu → Kas Kasir / Uang Masuk-Keluar**.
The opening cashier or manager may record movements; authenticated outlet staff
can read history. Optional evidence uses a signed URL bound to the actor, outlet, cash
session, and movement `request_id`. The client uploads directly to Supabase and sends
`proof_object_name` with the movement. PostgreSQL stores only the resulting URL; authorized users obtain
that URL through the protected proof endpoint and the frontend renders it directly.

Fresh backend installs must apply `migrations/000015_create_cash_movements.up.sql`
after 000014, followed by `migrations/000016_store_only_image_urls.up.sql` and
`migrations/000017_add_supabase_identity.up.sql`. Migration 000016 removes duplicate
object metadata so PostgreSQL retains
only `image_url`, `avatar_url`, and `proof_url`. It refuses to run while legacy local
product/avatar URLs or local cash-proof object keys still require migration. No real
cash record or till closure was created during verification. Existing frozen
closings are preserved. Pending uncertain writes must be recovered/retried with
the same intent; inspect history before re-entry after browser reload/logout.

See the generated OpenAPI and `api/bruno/cash-movements/` for API, permissions,
corrections, tests, and recovery behavior. General expense accounting remains
separate work.
