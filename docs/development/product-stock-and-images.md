# Firebase product images, stock and workbook exports

Firebase Authentication, Firebase Storage, and PostgreSQL are required. Existing
products remain untracked until a manager enables stock.

## Existing installation

Apply only `migrations/000009_add_product_stock.up.sql` to an existing database,
inside a transaction. Do not rerun `make migrate-up` against populated tables;
that target is for a fresh database. Back up the database before changing a shared
environment. The development workspace's local database already has migration 000009.

Configure `.env`:

```dotenv
FIREBASE_PROJECT_ID=dikasa-project-id
FIREBASE_WEB_API_KEY=replace-with-web-api-key
FIREBASE_STORAGE_BUCKET=dikasa-project-id.firebasestorage.app
GOOGLE_APPLICATION_CREDENTIALS=/secure/firebase-service-account.json
```

Restart the backend after changing environment values. The backend validates PNG/JPEG
content up to 3 MiB, 4096 px per side, and 16 megapixels, generates an object name, uploads to Firebase,
and persists the returned download URL. Product responses expose `image_url`; profile
responses expose `avatar_url`. Frontends render those URLs directly and never request
image bytes from this API.

Objects use these prefixes:

- `outlets/{outlet_id}/products/{product_id}/images/{uuid}.{ext}`
- `outlets/{outlet_id}/users/{user_id}/avatar/{uuid}.{ext}`
- `outlets/{outlet_id}/cash-sessions/{session_id}/proofs/{uuid}.{ext}`

Every image type uses the same shared uploader and receives a Firebase download URL.
PostgreSQL stores only that URL—never bytes, duplicate object names, tokens in separate
columns, MIME metadata, or hashes. Product/avatar responses return their URL normally;
cash-proof URLs require an authenticated same-outlet lookup.

Migration `000016_store_only_firebase_image_urls` fails safely if it finds legacy local
product/avatar URLs or cash-proof object keys. Upload those files to Firebase and update
their URL columns before rerunning the migration; it never discards an unresolved file reference.

## Upload and bucket decision

Uploads stay backend-mediated rather than using signed upload URLs. The current 3 MiB
limit is small, and the backend must inspect MIME, signature, dimensions, and decoded
image content before accepting a reference. A direct signed upload would require a
second finalize endpoint plus server-side object revalidation and orphan cleanup. Revisit
signed/resumable uploads only if file sizes or upload traffic become large enough to make
API bandwidth a demonstrated bottleneck.

Use a separate Firebase project per environment, with its default bucket:

- `dikasa-dev.firebasestorage.app`
- `dikasa-staging.firebasestorage.app`
- `dikasa-prod.firebasestorage.app`

Do not mix environments in one bucket merely by adding a folder prefix. Firebase
download URLs are bearer-style links: anyone with the URL can render the object until
it is deleted or its token is rotated. Product and avatar URLs are returned in ordinary
resources. Cash-proof URLs are disclosed only by the authorized proof endpoint and
must still be treated as sensitive.

## API contract

All paths below are relative to `/dikasa/v1`. See generated `api/openapi.yaml` for
the full schemas and `api/bruno/catalog/` for requests.

| Method/path | Access and behavior |
| --- | --- |
| `POST /products`, `PATCH /products/{id}` | Manager; multipart `image` uploads to Firebase and the response returns `image_url` |
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
