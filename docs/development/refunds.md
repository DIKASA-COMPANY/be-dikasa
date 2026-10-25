# Refunds and cancellation

Cashiers and managers can record a whole-remaining receipt refund or select original
receipt items and quantities. The original sale remains intact. This records money
already returned manually; it does not send money through a payment provider.

## Setup and cashier workflow

Existing databases require **only migration000011** after000010. Apply its up file
once with `psql -1 -v ON_ERROR_STOP=1 -f migrations/000011_add_transaction_refunds.up.sql`.
Do not reset application tables or rerun old migrations. New installations use the
Makefile sequence. Restart backend/Flutter after updating the source. No new
environment variables, Firebase services or package dependencies are required.

Open **Daftar Order → Dibayar → Detail**. Original, refunded and net amounts are
separate. Choose **Batalkan / Refund Seluruh Sisa** or **Refund Item**. Select item
quantities, optionally choose **Kembali ke stok**, enter a reason and actual return
method, then request **Pratinjau Refund**. Review the original-tax allocation and
total before **Catat Uang Sudah Dikembalikan**. A current outlet cash session is
required when recording a new refund, including refunds for older sales.

After a timeout, do not hand over money again. Keep the original request UUID/body:
use **Periksa Hasil Refund** or **Ulangi Permintaan yang Sama**. A lookup404 alone
does not prove failure because the original request may still be executing. After
an app restart, inspect receipt refund history before taking any further action.

Unpaid Hapus clears the local cart; deleting an unpaid saved order keeps its existing
soft-delete audit. Paid cancellation uses the full-remaining refund flow and records
status refunded, never deletion of the paid receipt. A refunded saved order stays
completed and cannot be reopened as an unpaid cart.

## HTTP contract

All endpoints require Bearer authentication and an outlet cashier/manager role under
`/dikasa/v1`. Missing and foreign-outlet IDs both return404.

| Method/path | Response |
| --- | --- |
| GET /transactions/{transaction_id} | Original snapshot, refund revision, line IDs/quantities and stock eligibility |
| GET /transactions/{transaction_id}/refunds | `{data:[],meta:{page,page_size,total}}`, default20/max100 |
| POST /transactions/{transaction_id}/refunds/preview | Read-only server amounts and preview_hash,200 |
| POST /transactions/{transaction_id}/refunds | Immutable successful refund,201 including replay |
| GET /refunds/by-request/{request_id} | Previously committed successful response,200 or unresolved404 |

Preview input:

```json
{
  "mode": "items",
  "method": "cash",
  "reason": "Unopened item returned",
  "expected_refund_revision": 0,
  "items": [{
    "transaction_item_id": "00000000-0000-4000-8000-000000000001",
    "quantity": 1,
    "restock_quantity": 0
  }]
}
```

Use actual original transaction-item UUIDs from receipt detail, never product IDs.
`mode` is full or items. Full mode includes every remaining line at its exact
remaining quantity; item mode selects a nonempty subset. All IDs must be unique.
Quantity is1..1000 and no greater than remaining units; return quantity is0..refund
quantity. Reason is1..500 Unicode characters. Methods are cash, wallet, debit_card,
qris or transfer; default to original supported method in the UI, with an explicit
choice required for unsupported historical point payments.

Confirmation adds a fresh `request_id` UUID and the `preview_hash` returned for this
exact selection. Do not send authoritative prices/amounts or change the reason,
method, quantities or stock decision after preview without requesting a new one.
Use a new UUID for each intentional refund; retain the same UUID/body for retry.

Stable conflict codes: REFUND_REVISI_BERUBAH, JUMLAH_REFUND_TIDAK_TERSEDIA,
REQUEST_ID_DIGUNAKAN, SESI_KAS_BELUM_AKTIF and STOK_PENGEMBALIAN_TIDAK_VALID.
Malformed selections use422; inconsistent/legacy snapshots use
TRANSAKSI_REFUND_TIDAK_DIDUKUNG422. Refund details are not disclosed across outlets.

## Amount and inventory rules

Refunds use the original charged total, not cash tender/change and not today's
catalog price or tax. Original discount and tax are allocated to receipt lines by
largest remainder, with original-item UUID tie-breaking. Successive quantities use
cumulative floor differences so partial refunds plus the final remainder equal the
original charged total exactly. Big-integer intermediate arithmetic prevents overflow.
Zero-price units may be refunded for Rp0; final status depends on remaining units.
Original tip/rounding metadata is not added again to the stored charged total.

Stock return starts at0. The original sale must have a recorded tracked deduction
for that parent product. A previously untracked sale cannot create inventory merely
because tracking was enabled later. Variations share the parent budget and balance.
Tracking must still be enabled; the balance cannot exceed1,000,000,000. Inactive
tracked products may receive returns without being reactivated. Refunds without
restocking remain possible when inventory is untracked. Stock-return movements
carry refund_id and original transaction_id in the audit history.

The receipt row and remaining-unit aggregates are locked for confirmation. Refund
header/items, optional stock balances/movements, receipt status and refund revision
commit together. Same-command replay is checked before current revision/session/
tracking checks, so later changes do not invalidate an already committed retry.

Order history preserves total_amount and adds refunded_amount, net_after_refund,
refund_revision and refunds_supported. Status partially_refunded means units remain;
refunded means every unit has been reversed. Historical refunded/cancelled rows
without a refund ledger show unknown totals rather than inventing past refunds.

## Audit, tests and limits

Migration000011 adds transaction_refunds and transaction_refund_items, scoped keys,
transaction refund_revision/status and stock_movements.refund_id/kind. The down
migration refuses to erase existing refund history and is not a routine undo action.
The Draw.io source documents these changes. Bruno examples cover every endpoint.

Run `go test ./...` and generate OpenAPI with `go generate ./internal/server`.
Opt in to guarded temporary-schema integration with a loopback
DIKASA_TEST_DATABASE_URL, then `go test ./internal/refund ./migrations -count=1 -v`.
These tests create their own sales/refunds and clean only their random schema;
they do not refund application/user transactions. Flutter has corresponding
service/controller/widget/mock tests and optional CAPTURE_REFUNDS screen capture.

Mock refunds are memory-only. Their preview token is deterministic for demo
validation; the real API uses SHA-256 and PostgreSQL locking. Both use original
budgets, big-integer arithmetic, quantity limits and explicit inventory returns.

No gateway payouts, arbitrary goodwill amounts, refund reversal, exchanges,
split-tender allocation, customer management, cash closing or accounting reports
are implemented by this increment. Review the dated workspace implementation log
for actual verification counts, migration application and remaining limitations.
