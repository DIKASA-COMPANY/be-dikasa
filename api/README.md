# Dikasa API Artifacts

- `openapi.yaml` is generated from the Huma operations. Do not edit it manually;
  run `make openapi` or `go generate ./internal/server`.
- `bruno/` contains the local environment and executable example requests for
  manual API testing.
- Swagger UI is served at `/dikasa/v1/docs` while the API is running.

All application endpoints use the `/dikasa/v1` prefix. Login, registration,
registration OTP confirmation/resend, Swagger, and OpenAPI are public. Other
endpoints, including cash sessions, products, product categories, order types, tables, reservations, orders, and customers, require
`Authorization: Bearer <access_token>` using a Supabase access token. Login validates
the PostgreSQL credential, imports the existing UUID and bcrypt hash when needed,
and returns a Supabase access/refresh session. `supabase_user_id` binds password or
OAuth identities to the PostgreSQL user.

The payment screen reads the current outlet tax from
`GET /dikasa/v1/payments/tax-percentage`. The response contains only the
numeric `tax_percentage`; `10` means 10 percent, not `0.10`. Cart parsing and
all tax/total calculations remain the frontend's responsibility.

Wallet options are outlet-scoped. `GET /dikasa/v1/wallets` supplies the active
dropdown values; managers use POST/PATCH/DELETE on `/wallets` to manage them.
Delete is a soft delete so historical payments keep their wallet reference.

`POST /dikasa/v1/transactions` runs only after the user confirms payment. The
frontend sends its monetary calculation together with product snapshots,
quantities, notes, and optional variants. The backend validates arithmetic and
scope in Go, attaches the outlet's active cash session, generates the
transaction number, and commits the header plus every item atomically.

`GET /dikasa/v1/orders` reads transaction snapshots for the authenticated
user's outlet. It supports case-insensitive `search` on receipt number/customer
name, optional `order_type_id` and `status`, plus `page`/`page_size`. The response
includes the order-type and optional restaurant-table labels needed by the order
list screen.

`GET /dikasa/v1/customers` returns active registered customers from the same
outlet. It supports case-insensitive `search` on name/phone/email, optional
`gender_id`, and pagination. Customer profiles are separate from staff `users`;
transactions retain a `customer_name` snapshot so historical receipts do not
change when a customer profile changes.

Catalog resources use `/products` (never `/foods`, `/drinks`, or `/list`). Use
the exact query key `searchByProductName` for case-insensitive partial name
searches. Product writes are multipart requests and are restricted to managers.
The Figma-aligned product form uses `product_type`, `selling_price`, optional
online/cost prices, `stock_status`, an optional JSON `variations` part, and optional
`image_object_name`. Obtain that object name from `POST /products/image-upload-intents`
and upload bytes directly to the returned Supabase URL before the mutation.
Responses retain `price` as an alias of `selling_price` for existing POS clients.

Reservation resources use `/reservation-types`, `/restaurant-tables`, and
`/reservations`. Cashiers and managers can create and manage reservations in
their own outlet; only managers can configure restaurant tables. The
`print_reservation` and `send_reservation` flags are stored without invoking an
external print or messaging provider.

Account settings use `POST /users/me/avatar-upload-intents` followed by direct
Supabase upload and `PATCH /users/me` with `avatar_object_name` for avatar changes,
`PATCH /users/me/pin` for six-digit PIN changes, and
`PATCH /users/me/password` for password changes. Role and outlet are read-only.

See the root `README.md` for the full local setup and Bruno walkthrough.
Run `make migrate-supabase-users` after migration 000017 to import active bcrypt
accounts in bulk; `/login` retains a rolling-migration fallback.

Product photos, profile avatars, and cash proofs use server-issued Supabase signed
upload URLs. Clients upload bytes directly to Storage; the backend then downloads a
bounded object, validates the real PNG/JPEG content, and persists its public URL.
The API never accepts or proxies image bytes. Cash-proof URLs remain available only
through the authenticated same-outlet proof lookup.
PostgreSQL remains the primary database and persists only each resulting URL, never
file bytes or duplicate object metadata. Shared quantitative stock,
deduplicated stock adjustments/history, and manager XLSX exports are described in
[`docs/development/product-stock-and-images.md`](../docs/development/product-stock-and-images.md).
Transactions now reject stale catalog/variant prices or unavailable stock with
specific 409 errors. Regenerate OpenAPI with `go generate ./internal/server` after
contract changes. Bruno stock request IDs must be fresh per intended action and
reused unchanged only for retries of the same command.

Saved-cart endpoints and optional draft checkout association are documented in
[Saved unpaid orders](../docs/development/saved-orders.md). Bruno `saved-orders/`
includes create/list/detail/update/delete, immutable replay and stale-conflict examples.

Refund detail/preview/confirmation/history/recovery endpoints and original-value
allocation rules are documented in [refunds.md](../docs/development/refunds.md).
Bruno `refunds/` includes whole/item, replay and stale-preview examples.

Cash-session closing, reconciliation and recovery: [Cash reconciliation guide](../docs/development/cash-reconciliation.md).

Four sales reports and XLSX: [Reporting guide](../docs/development/sales-reports.md).
