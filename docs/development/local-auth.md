# Local development without Firebase

Run Go and PostgreSQL locally. Firebase, SMTP and object storage are unnecessary
for seeded-account login, profile access and text-only catalog operations.

## First setup

1. Copy `.env.local.example` to `.env` if you do not already have a local config.
2. Set `DATABASE_URL` to your local PostgreSQL database. Generate a unique
   `LOCAL_AUTH_SECRET` with at least 32 random bytes (64 hexadecimal characters):

   ```powershell
   [guid]::NewGuid().ToString('N') + [guid]::NewGuid().ToString('N')
   ```

3. Download dependencies with `go mod download`.
4. Apply migrations **once to a new empty database**, in numeric order. Each
   file runs in its own transaction, stopping on failure:

   ```powershell
   $env:DATABASE_URL = 'postgres://dikasa:YOUR_PASSWORD@localhost:5432/dikasa?sslmode=disable'
   Get-ChildItem .\migrations\*.up.sql | Sort-Object Name | ForEach-Object {
       psql --no-password $env:DATABASE_URL -v ON_ERROR_STOP=1 -1 -f $_.FullName
       if ($LASTEXITCODE -ne 0) { throw "Migration failed: $($_.Name)" }
   }
   ```

   The migrations are not rerunnable; use only pending migrations on an existing
   database. URL-encode special characters in the connection-string password.

5. Explicitly create the development account and sample outlet/catalog:

   ```powershell
   go run ./cmd/seed-local
   ```

   The new account is `admin` / `dikasa123`, with manager permissions at
   **Dikasa Local**. It includes a sample Nasi Goreng product, a takeaway order
   type and a GoPay wallet. To choose a different initial password, set
   `LOCAL_SEED_PASSWORD` before seeding. An existing `admin` is never modified.
   This command accepts only `AUTH_MODE=local` and a loopback PostgreSQL URL;
   connection query overrides are rejected.

## Start each day

In the backend folder:

```powershell
go run ./cmd/api
```

The local server listens on **127.0.0.1:8081** when `.env` uses `PORT=8081`.
Swagger: <http://localhost:8081/dikasa/v1/docs>.
Bruno: select the `local-auth` environment and run Login, then My Profile.

In `mobile-dikasa/.env`:

```dotenv
API_BASE_URL=http://localhost:8081/dikasa/v1
USE_MOCK_API=false
```

In a second terminal, from the mobile folder:

```powershell
flutter run -d web-server --web-hostname=127.0.0.1 --web-port=8080
```

Open <http://127.0.0.1:8080>. Fully restart Flutter after changing `.env`.
Only `http://localhost:8080` and `http://127.0.0.1:8080` are allowed as cross-origin
browser clients by default in local mode. Set `CORS_ALLOWED_ORIGINS` to a
comma-separated list of exact origins if you change the frontend address.
Same-origin Swagger requests remain available in either auth mode.

## Session and feature limits

- PostgreSQL bcrypt passwords and normal outlet permissions still apply.
- Local access tokens expire after one hour. There is no local refresh endpoint;
  log in again. `refresh_token` is empty.
- The legacy `firebase_uid` response field contains the PostgreSQL user UUID in
  both modes. It does not indicate that Firebase was contacted in local mode.
- `MAIL_MODE=disabled` returns 503 from registration, resend and confirmation
  before creating or modifying accounts. To test real email OTP locally, choose
  `MAIL_MODE=smtp` and configure SMTP plus `OTP_SECRET`.
- `IMAGE_STORAGE_MODE=disabled` keeps uploads unavailable.
- The mobile checkout now also connects opening cash, order types, manual
  payment recording, order history and read-only customer browsing. See the
  mobile README for its payment-reconciliation limits and opt-in live test.

## Switch to Firebase later

In the backend `.env`, set:

```dotenv
AUTH_MODE=firebase
MAIL_MODE=smtp
FIREBASE_PROJECT_ID=your-project-id
FIREBASE_WEB_API_KEY=your-web-api-key
GOOGLE_APPLICATION_CREDENTIALS=C:/path/outside/repository/service-account.json
OTP_SECRET=your-random-secret-at-least-32-characters
SMTP_HOST=your-host
SMTP_PORT=587
SMTP_USERNAME=your-username
SMTP_PASSWORD=your-password
SMTP_FROM=your-sender@example.com
```

Keep `IMAGE_STORAGE_MODE=disabled` until storage is separately configured.
Restart the API and log in again. PostgreSQL accounts, passwords and business
data stay in place; local tokens are no longer accepted. When `AUTH_MODE` is
omitted, Firebase remains the default and missing credentials cause a startup
error rather than a fallback to local authentication.

## Verification

```powershell
go test ./...
go vet ./...
```

For the real PostgreSQL migration regression test, set
`DIKASA_TEST_DATABASE_URL` to a local test connection and run
`go test ./migrations -run TestMigrationsApplyToPostgres -v`. It applies every
migration inside a temporary schema and rolls the transaction back, leaving
existing tables and data untouched. Without that environment variable the
integration test is skipped.
