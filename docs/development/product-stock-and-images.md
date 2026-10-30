# Supabase product images, stock and workbook exports

Supabase Auth, Supabase Storage, and PostgreSQL are required. Existing
products remain untracked until a manager enables stock.

## Existing installation

Apply only `migrations/000009_add_product_stock.up.sql` to an existing database,
inside a transaction. Do not rerun `make migrate-up` against populated tables;
that target is for a fresh database. Back up the database before changing a shared
environment. The development workspace's local database already has migration 000009.

Configure `.env`:

```dotenv
SUPABASE_URL=https://your-project-ref.supabase.co
SUPABASE_PUBLISHABLE_KEY=sb_publishable_replace_me
SUPABASE_SECRET_KEY=sb_secret_replace_me
SUPABASE_STORAGE_BUCKET=dikasa-images
```

Restart the backend after changing environment values. The backend validates declared
PNG/JPEG type and size, generates an actor/outlet-scoped object name, and returns a
Supabase signed upload URL. The client uploads directly to Supabase. During the business
mutation the backend downloads at most 3 MiB, validates the real signature, MIME,
4096-pixel axis and 16-megapixel limits, then persists the public URL. Product responses expose `image_url`; profile
responses expose `avatar_url`. Frontends render those URLs directly and never request
image bytes from this API.

Objects use these prefixes:

- `outlets/{outlet_id}/users/{actor_id}/product-images/{uuid}.{ext}`
- `outlets/{outlet_id}/users/{user_id}/avatar/{uuid}.{ext}`
- `outlets/{outlet_id}/cash-sessions/{session_id}/users/{actor_id}/requests/{request_id}/proofs/{uuid}.{ext}`

Every image type uses the same shared signer/completion validator and receives a Supabase public URL.
PostgreSQL stores only that URL—never bytes, duplicate object names, tokens in separate
columns, MIME metadata, or hashes. Product/avatar responses return their URL normally;
cash-proof URLs require an authenticated same-outlet lookup.

Migration `000016_store_only_image_urls` fails safely if it finds legacy local
product/avatar URLs or cash-proof object keys. Upload those files to Supabase and update
their URL columns before rerunning the migration; it never discards an unresolved file reference.
Migration `000017_add_supabase_identity` repeats the public-URL guard for upgraded
databases and adds the unique nullable `users.supabase_user_id` binding.

## Signed upload flow and cleanup

All image bytes bypass the DIKASA API:

1. Call the feature upload-intent endpoint with `content_type` and `size`.
2. `PUT` the bytes directly to the returned `upload_url` using the same content type.
3. Send the returned `object_name` in the product, profile, or cash-movement mutation.
4. The backend authorizes the same actor/outlet/resource binding, downloads the object
   with a strict limit, validates its real content, and only then persists the public URL.

Signed upload URLs currently expire after 7200 seconds and do not permit overwrite.
Request a new intent after expiry; never retry with another actor's object name. Invalid
objects and objects whose database adoption fails are deleted immediately. Operations
must also run a scheduled reconciliation after the two-hour intent window: list image
objects, retain every URL referenced by PostgreSQL, and delete older unreferenced objects.
Do not apply an age-only bucket policy because live and abandoned objects share scoped prefixes.

Use a separate Supabase project per environment, with a public bucket such as:

- `dikasa-images-dev`
- `dikasa-images-staging`
- `dikasa-images-prod`

Do not mix environments in one bucket merely by adding a folder prefix. Public Storage
URLs can be rendered by anyone who knows the URL until the object is deleted. Product and avatar URLs are returned in ordinary
resources. Cash-proof URLs are disclosed only by the authorized proof endpoint and
must still be treated as sensitive.

## API contract

All paths below are relative to `/dikasa/v1`. See generated `api/openapi.yaml` for
the full schemas and `api/bruno/catalog/` for requests.

| Method/path | Access and behavior |
| --- | --- |
| `POST /products/image-upload-intents` | Manager; returns a 2-hour signed Supabase upload URL and actor-bound `object_name` |
| `POST /products`, `PATCH /products/{id}` | Manager; optionally adopts a completed direct upload through `image_object_name` and returns `image_url` |
| `GET /products/export.xlsx` | Manager; XLSX attachment for all matching catalog pages |
| `POST /products/{id}/stock` | Manager; enable/disable tracking or update its threshold |
| `POST /products/{id}/stock-adjustments` | Manager; change a tracked quantity with a reason |
| `GET /products/{id}/stock-movements` | Manager; paginated audit history (`page`, `page_size`, maximum 100) |
| `POST /transactions` | Existing cashier access; now checks current products, variants, prices and availability and deducts tracked quantities atomically |

Example tracking setup:

```json
{
  "request_id": "11111111-1111-4111-8111-111111111111",
  "reason": "Initial stock count",
  "track_stock": true,
  "initial_quantity": 20,
  "low_stock_threshold": 5
}
```

Generate a new UUID for each intended configuration/adjustment. Reuse the same UUID
and identical body only to retry that same command. The backend returns the original
movement without changing stock twice; a different payload under the same UUID is
a 409 conflict. Quantities/thresholds are whole units from 0 to 1,000,000,000.
Adjustments use signed `quantity_delta` within that range, excluding zero, and a
required reason of at most 500 characters. Negative balances are rejected.

`initial_quantity` is accepted only when enabling tracking that was disabled. Once
tracking is enabled, use adjustments; sending a stale initial quantity cannot reset
a live balance. Disabling tracking retains the quantity and history. Ordinary product
PATCH operations never write any stock columns.

Catalog responses now include `track_stock`, `stock_quantity`, `low_stock_threshold`
and `effective_stock_status`. The existing `stock_status` remains the manual pause
switch. Effective status is `out_of_stock` for a pause or tracked zero, `low_stock`
for a positive tracked balance at/below the threshold, otherwise `available`.
Base products and all variations consume one shared product balance.

Workbook filters: `product_id`, `product_type`, `category_id`,
`searchByProductName`, `availability`, and `include_inactive`. It contains a base
row and active variation rows. Quantity/threshold appear only on the base row;
blank optional prices stay blank and user strings remain literal text cells.

Transaction items continue to use the real `product_id`, optional
`product_variant_id`, `variant_name`, and selected `unit_price`. Catalog/variant
changes produce `KATALOG_BERUBAH` (409); insufficient quantity produces
`STOK_TIDAK_CUKUP` (409). The receipt, item rows, balance deductions and sale movements
are committed or rolled back together. All product locks use stable ID ordering.
The existing uncertain-payment/manual-reconciliation flow remains necessary after
a lost response; no cross-restart exactly-once payment guarantee is added.

## Tests and architecture

```powershell
go test ./...
go generate ./internal/server
```

Set `DIKASA_TEST_DATABASE_URL` to a loopback test database URL to opt into
`TestSharedStockPostgres` and migration integration. The stock test creates a
random temporary schema, applies all migrations there, tests adjustment deduplication,
stale edits, last-unit concurrency and rollback, then drops only that test schema.
It does not reset application data. Never print a real DSN in logs or commit it.

The Draw.io ERD includes a migration-000009 page with product columns,
`stock_movements`, keys, constraints and actor/receipt relationships.
