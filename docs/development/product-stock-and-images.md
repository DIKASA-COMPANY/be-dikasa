# Local product images, stock and workbook exports

These features work with the existing local-auth setup and PostgreSQL. Firebase
is not required. Existing products remain untracked until a manager enables stock.

## Existing installation

Apply only `migrations/000009_add_product_stock.up.sql` to an existing database,
inside a transaction. Do not rerun `make migrate-up` against populated tables;
that target is for a fresh database. Back up the database before changing a shared
environment. The development workspace's local database already has migration 000009.

Configure `.env`:

```dotenv
IMAGE_STORAGE_MODE=local
LOCAL_IMAGE_DIRECTORY=uploads
PUBLIC_BASE_URL=http://127.0.0.1:8081
```

Restart the backend after changing code or environment values. Files persist under
the configured directory, which must be backed up alongside PostgreSQL. The default
`uploads/` directory is ignored by Git. `PUBLIC_BASE_URL` is a trusted HTTP(S) origin,
not a path or an incoming request header. Product images are public using random
server-generated filenames. Upload, replacement and removal require manager access.
Only PNG/JPEG up to 3 MiB and 24 megapixels are accepted; no directory listing or
arbitrary filesystem path is exposed.

`IMAGE_STORAGE_MODE=firebase` still selects the existing Firebase adapter. Changing
providers does not copy existing files; keep old image hosting available until an
explicit migration is performed. User-avatar storage is outside this increment.

## API contract

All paths below are relative to `/dikasa/v1`. See generated `api/openapi.yaml` for
the full schemas and `api/bruno/catalog/` for requests.

| Method/path | Access and behavior |
| --- | --- |
| `POST /products`, `PATCH /products/{id}` | Manager; existing multipart `image` and `remove_image` fields now work in local mode |
| `GET /product-images/{filename}` | Public; only generated, regular local product image files |
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
