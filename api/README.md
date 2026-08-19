# Dikasa API Artifacts

- `openapi.yaml` is generated from the Huma operations. Do not edit it manually;
  run `make openapi` or `go generate ./internal/server`.
- `bruno/` contains the local environment and executable example requests for
  manual API testing.
- Swagger UI is served at `/dikasa/v1/docs` while the API is running.

All application endpoints use the `/dikasa/v1` prefix. Login, registration,
registration OTP confirmation/resend, Swagger, and OpenAPI are public. Other
endpoints, including cash sessions, products, product categories, order types, tables, and reservations, require
`Authorization: Bearer <Firebase ID token>`.

Catalog resources use `/products` (never `/foods`, `/drinks`, or `/list`). Use
the exact query key `searchByProductName` for case-insensitive partial name
searches. Product writes are multipart requests and are restricted to managers.
The Figma-aligned product form uses `product_type`, `selling_price`, optional
online/cost prices, `stock_status`, and an optional JSON `variations` part.
Responses retain `price` as an alias of `selling_price` for existing POS clients.

Reservation resources use `/reservation-types`, `/restaurant-tables`, and
`/reservations`. Cashiers and managers can create and manage reservations in
their own outlet; only managers can configure restaurant tables. The
`print_reservation` and `send_reservation` flags are stored without invoking an
external print or messaging provider.

Account settings use `PATCH /users/me` for multipart profile/avatar changes,
`PATCH /users/me/pin` for six-digit PIN changes, and
`PATCH /users/me/password` for password changes. Role and outlet are read-only.

See the root `README.md` for the full local setup and Bruno walkthrough.
