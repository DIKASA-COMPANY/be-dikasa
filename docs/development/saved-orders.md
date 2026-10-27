# Saved unpaid orders

Cashiers and managers can park carts in PostgreSQL without Firebase. Orders are
shared within the authenticated outlet, survive server/app restarts and do not
reserve or deduct stock. Only successful payment changes inventory.

## Local setup

Apply migration `000010_add_saved_orders.up.sql` once to an existing database,
using `psql -1 -v ON_ERROR_STOP=1 -f ...`. Do not reapply old migrations or reset
tables. New installations use the updated Makefile migration order. Restart the
backend and Flutter after updating code. Existing configuration is sufficient.

## Cashier flow

1. Choose order type and items, then **Simpan**. Enter an optional label.
2. Confirmed save clears the cart. On a timeout the cart is frozen: **Coba ulang
   permintaan yang sama** sends the same command UUID and original body.
3. **Daftar Order → Tersimpan** lists outlet drafts; search by label, open or delete.
   The working cart must be empty before reopening a different saved order.
4. Current catalog prices/types are refreshed. Review changes explicitly before
   accepting new prices or removing unavailable catalog references. Cancelling
   this review leaves the persisted order untouched. Low stock is not reserved.
5. Pay normally. Edited/repriced draft lines are saved at a new confirmed revision
   before checkout. Select a table at payment time as before.
6. If payment is uncertain, **Periksa Daftar Order** checks the linked draft status.
   A paid draft provides its receipt UUID; an open draft permits deliberate retry.
   A deleted/inaccessible draft stays blocked until the cashier reconciles history.

Clearing a reopened cart detaches it locally; the saved order remains available
until paid or explicitly deleted. There is no automatic offline cart backup.

## HTTP contract

All endpoints require Bearer authentication under `/dikasa/v1`:

| Method/path | Behavior |
| --- | --- |
| POST /saved-orders | Create/replay; request_id, label, order_type_id, items |
| GET /saved-orders | Open outlet drafts; search, page>=1, page_size 1..100 |
| GET /saved-orders/{saved_order_id} | Detail, including paid/deleted status and transaction_id |
| PUT /saved-orders/{saved_order_id} | Replacement cart plus request_id and expected_revision |
| DELETE /saved-orders/{saved_order_id} | Soft delete; request_id and expected_revision in query |

Mutations return200 with `{data: ...}`; list returns `{data: [], meta:
{page,page_size,total}}`, matching existing POS lists. Server-owned line positions
are optional on input and ignored/recomputed. Items carry product_id, optional
product_variant_id, product_name, optional variant_name, unit_price, quantity and
optional notes. Up to100 unique product/variant lines, quantity1..1000, label100
Unicode characters, notes500; integer amounts bounded to9007199254740991. The
estimate is pre-tax. Outlet and creator IDs always come from the authenticated
account. Inactive-but-existing references can be retained in drafts for review.

`POST /transactions` accepts optional `saved_order_id` and
`saved_order_revision`, required together. The cart/order type must match that
revision. Current catalog/price/stock validation still applies. Draft completion,
receipt/items and stock deductions commit together; any failure rolls back all.

| Status/code | Client action |
| --- | --- |
| 401/403 | Log in / check cashier or manager outlet assignment |
| 404 | Missing or inaccessible outlet draft; no cross-outlet disclosure |
| 409 PESANAN_TERSIMPAN_BERUBAH | Preserve cart, reload latest draft before editing |
| 409 REQUEST_ID_DIGUNAKAN | UUID was reused with changed actor/payload; do not alter a retry |
| 409 PESANAN_SUDAH_DIBAYAR | Fetch draft detail and use its existing receipt reference |
| 409 KATALOG_BERUBAH / STOK_TIDAK_CUKUP | Review product/variation/price/stock |
| 422 | Correct input or catalog reference |
| timeout/5xx/malformed success | Keep exact command UUID/body and resolve/retry |

For stale local carts, save any separate work first, clear the cart, then reopen
the draft from Tersimpan. Never silently replace the latest server version.

## Replay and locking

Every new intended create/update/delete uses a new UUID. Retry with the same UUID
and exact payload. Equivalent UUID spellings are canonicalized. The server takes
a transaction advisory lock on outlet/request identity, checks the immutable
command journal, then locks the target draft row. Successful retries return the
original snapshot even after later edits; errors do not consume the UUID.

Draft mutations and checkout share row locks and revision checks; one competing
edit/delete/payment can win. Checkout locks draft first, then product rows in
the catalog's stable order. Completed/deleted rows remain for audit and recovery.
Paid receipts have a unique draft link. This protection applies to linked drafts;
ordinary unsaved transactions keep their existing manual reconciliation flow.

## Verification

`go test ./...` covers domain, authorization, Huma payloads, payment association and
existing regressions. Set `DIKASA_TEST_DATABASE_URL` to an explicitly local URL to
run `go test ./internal/savedorder ./migrations -count=1 -v`. Integration uses random
temporary schemas, never resets public application tables, and tests concurrent
create/replay/edit/delete/payment plus failed-payment rollback.

Bruno `saved-orders/` contains the normal flow and explicit replay/stale-conflict
examples. Choose active product/order-type IDs from your outlet, set prices/tax
to actual current values and open a cash session before paying. Delete and payment
are alternative endings; an already paid draft cannot be deleted. Static example
request UUIDs must be replaced for each new intentional action.

Flutter mock mode keeps drafts only in memory and mirrors revision/replay/payment
rules. The PostgreSQL path provides real persistence. The ERD adds a saved-orders
page with the actual migration000010 tables, composite keys and constraints.
