# Dikasa Backend

Dikasa Backend is a Go API for restaurant operations. The current implementation
provides business-owner registration with email OTP verification, PostgreSQL
credential validation, Firebase token issuance, authenticated POS catalogs,
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
- Firebase Authentication for ID and refresh tokens
- Firebase Storage for optional product images (staged behind a runtime mode)
- SMTP for registration OTP delivery
- `go-playground/validator` for request validation
- Bruno for manual API testing
- Draw.io for the database ERD

## How authentication works

PostgreSQL is the source of truth for users, business data, and password hashes.
Firebase does not store Dikasa business data or passwords.

The login flow is:

1. The API finds the user in PostgreSQL by username, phone number, or email.
2. The submitted password is compared with the PostgreSQL bcrypt hash.
3. The API requests a Firebase custom token using `users.id` as the Firebase UID.
4. The custom token is exchanged for a Firebase ID token and refresh token.
5. Protected endpoints verify the Firebase ID token and use its UID to load the
   PostgreSQL user.

The registration flow creates an inactive outlet and user. Both are activated
only after the selected email address is verified with an OTP.

## Project structure

```text
be-dikasa/
├── cmd/
│   ├── api/                    # Application entry point and dependency wiring
│   └── openapi/                # OpenAPI generator
├── internal/
│   ├── auth/                   # Login and Firebase token issuance
│   ├── cash/                   # Opening-cash sessions
│   ├── catalog/                # Products, categories, order types, image storage
│   ├── config/                 # Environment configuration
│   ├── database/               # PostgreSQL and Firebase clients
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
- A Firebase project with Firebase Authentication enabled.
- A Firebase service-account JSON file for local Application Default
  Credentials.
- The Firebase Web API key for custom-token exchange.
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

### 3. Prepare Firebase

From the Firebase project used by your team:

1. Enable Firebase Authentication.
2. Obtain the Firebase project ID and Web API key.
3. Download a service-account JSON file for local development.
4. Store the JSON outside the repository. Never commit it.

The Firebase UID is intentionally the same UUID as `users.id` in PostgreSQL.

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
$env:FIREBASE_PROJECT_ID = "your-firebase-project-id"
$env:FIREBASE_WEB_API_KEY = "your-firebase-web-api-key"
$env:IMAGE_STORAGE_MODE = "disabled"
$env:FIREBASE_STORAGE_BUCKET = "your-firebase-project-id.firebasestorage.app"
$env:GOOGLE_APPLICATION_CREDENTIALS = "C:\secure\firebase-service-account.json"
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
export FIREBASE_PROJECT_ID='your-firebase-project-id'
export FIREBASE_WEB_API_KEY='your-firebase-web-api-key'
export IMAGE_STORAGE_MODE='disabled'
export FIREBASE_STORAGE_BUCKET='your-firebase-project-id.firebasestorage.app'
export GOOGLE_APPLICATION_CREDENTIALS='/secure/firebase-service-account.json'
export OTP_SECRET='replace-with-a-random-secret-of-at-least-32-characters'
export SMTP_HOST='smtp.example.com'
export SMTP_PORT='587'
export SMTP_USERNAME='mailer@example.com'
export SMTP_PASSWORD='your-smtp-password'
export SMTP_FROM='Dikasa <mailer@example.com>'
```

`OTP_SECRET` must be at least 32 characters and must be different for each
environment. Never commit real database, Firebase, SMTP, or OTP credentials.

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
```

PowerShell users can replace `"$DATABASE_URL"` with `$env:DATABASE_URL`.

The migrations create the `roles`, `genders`, `outlets`, `users`,
`registration_otps`, `cash_sessions`, `product_types`, `product_categories`,
`products`, and `order_types` tables. Passwords are stored as bcrypt
hashes. OTP values are stored as HMAC-SHA256 hashes and are never persisted as
plaintext. Opening cash is stored as a whole-Rupiah integer per outlet cash
session, with at most one open session per outlet.

`IMAGE_STORAGE_MODE=disabled` is the safe default: products without images and
text-only profile edits remain available, while image uploads return HTTP `503`. Set it to `firebase` only
after the bucket, Blaze billing, and Application Default Credentials are ready.
Images are limited to 3 MiB and validated as real PNG/JPEG content.
The same storage mode applies to optional profile avatars. PINs use six digits,
and both PINs and passwords are persisted only as bcrypt hashes.

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
| `POST` | `/login` | Public | Validate PostgreSQL credentials and issue Firebase tokens |
| `GET` | `/users/me` | Firebase Bearer token | Return the authenticated PostgreSQL profile |
| `PATCH` | `/users/me` | Firebase Bearer token | Partially update name, phone, email, or profile avatar |
| `PATCH` | `/users/me/pin` | Firebase Bearer token | Set or change the six-digit account PIN |
| `PATCH` | `/users/me/password` | Firebase Bearer token | Verify the current password and set a new password |
| `POST` | `/cash-sessions` | Firebase Bearer token | Open the outlet cash session with an opening-cash amount |
| `GET` | `/cash-sessions/current` | Firebase Bearer token | Return the outlet's active cash session |
| `GET` | `/payments/tax-percentage` | Firebase Bearer token | Return only the current outlet tax percentage for frontend payment calculation |
| `GET/POST` | `/wallets` | User / Manager | List active wallet options or create an outlet wallet |
| `PATCH/DELETE` | `/wallets/{wallet_id}` | Manager | Update or soft-delete an outlet wallet |
| `POST` | `/transactions` | User outlet with active cash session | Atomically save the paid transaction and all product snapshots |
| `GET` | `/orders` | Firebase Bearer token | List outlet orders with receipt/customer search, type/status filters, and pagination |
| `GET` | `/customers` | Firebase Bearer token | List active registered customers with search, gender filter, and pagination |
| `GET` | `/product-types` | Firebase Bearer token | Return Food, Drink, and Additional tabs |
| `GET` | `/products` | Firebase Bearer token | Return outlet products with category, type, search, and pagination filters |
| `GET` | `/products/{product_id}` | Firebase Bearer token | Return one outlet product |
| `POST/PATCH/DELETE` | `/products` and `/products/{product_id}` | Manager | Manage products with soft delete |
| `GET/POST` | `/product-types/{product_type}/categories` | User / Manager | List or create outlet categories for a product type |
| `PATCH/DELETE` | `/product-types/{product_type}/categories/{category_id}` | Manager | Update or soft-delete a category |
| `GET/POST` | `/order-types` | User / Manager | List or create outlet order types |
| `PATCH/DELETE` | `/order-types/{order_type_id}` | Manager | Update or soft-delete an order type |
| `GET` | `/reservation-types` | Firebase Bearer token | Return the seeded reservation-type dropdown |
| `GET` | `/restaurant-tables` | Firebase Bearer token | Return active outlet tables; optionally filter availability by `reservation_at` |
| `POST/PATCH/DELETE` | `/restaurant-tables` and `/restaurant-tables/{table_id}` | Manager | Configure outlet tables with soft delete |
| `GET/POST` | `/reservations` | User outlet | List or create outlet reservations |
| `GET/PATCH/DELETE` | `/reservations/{reservation_id}` | User outlet | Read, update, or cancel an outlet reservation |
| `GET` | `/health` | Firebase Bearer token | Confirm the authenticated API path is healthy |
| `GET` | `/docs` | Public | Serve Swagger UI |
| `GET` | `/openapi.yaml` | Public | Serve the generated OpenAPI document |

Product writes use `multipart/form-data`. The create form follows the product
screen: `name`, `category_id`, `product_type`, `selling_price`, `stock_status`,
optional `online_selling_price`, `cost_price`, `online_cost_price`, `description`,
and `image`. The optional `variations` part is `application/json` containing an
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

The application validates all required variables during startup. Compare your
terminal environment with `.env.example`. Remember that the application does
not automatically load `.env`.

### PostgreSQL migration fails on `pgcrypto`

The database user must be allowed to create or use the `pgcrypto` extension.
Ask the database administrator to enable it when using a managed database.

### Firebase initialization fails

Check `FIREBASE_PROJECT_ID` and verify that
`GOOGLE_APPLICATION_CREDENTIALS` points to a readable service-account JSON file
for the same project.

### Login returns a Firebase provider error

Check `FIREBASE_WEB_API_KEY`, confirm that Firebase Authentication is enabled,
and confirm that the service account can create custom tokens.

### Registration returns `EMAIL_GAGAL_DIKIRIM`

Check the SMTP host, port, authentication credentials, sender address, and the
provider's STARTTLS requirements. The pending registration remains available;
after the cooldown, use the resend endpoint with the selected verification
email.

### Protected requests return HTTP 401

Run **Login** in Bruno again and make sure the selected environment contains a
non-empty `accessToken`. Firebase ID tokens expire and must be refreshed or
issued again.

### Registration returns HTTP 409

At least one unique value is already registered. Use a different username,
owner phone, owner email, or business email for a new local test registration.
