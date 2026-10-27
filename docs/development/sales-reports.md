# Sales reports: activity dates and original snapshots

Four read-only views use original receipts and immutable refund records: summary,
payment methods, products/variants and original categories. Active outlet cashiers
and managers may read/export. No all-outlet scope or historical profit estimate.

## HTTP and dates

GET /dikasa/v1/reports/summary, /methods, /products, /categories. Each returns
{data:{kind,totals,daily,methods,rows},meta:{start_date,end_date,timezone,generated_at,
query,sort,page,page_size,total}}; inapplicable arrays are empty. GET
/dikasa/v1/reports/{report_kind}/export.xlsx returns a real Excel attachment using
all matching rows. Export ignores page slicing and is freshly calculated at its
own timestamp. Original JSON amounts are safe whole-Rupiah integers.

Parameters: start_date/end_date are inclusive YYYY-MM-DD in Asia/Jakarta (WIB),
default today, max366 days. Convert to [local start midnight, midnight after end)
UTC, not browser/device local dates. query matches displayed original product/
variation/category names (max200 characters), only on product/category reports.
sort is quantity_desc (net units), net_desc or name_asc, with stable identity ties.
page defaults1, page_size20/max100. Totals reflect full matching groups before pages.

Missing auth401; inactive/non-cashier/non-manager403; invalid filter422
FILTER_LAPORAN_TIDAK_VALID; unsupported/overflow422 DATA_LAPORAN_TIDAK_DIDUKUNG.
Internal errors use the shared sanitized envelope. Every SQL predicate is outlet
scoped; original parent receipts from another outlet cannot enter a report.

## Definitions

Sales are original paid_at events, independent of later status changes. Refunds
are their own created_at events; Wednesday refunds of Monday sales deduct Wednesday.
Original parents outside the interval supply item/category evidence only. A QRIS
sale refunded by cash remains a QRIS sale and cash-return event. Do not infer cash
drawer totals from this report; cash-session reconciliation is separately frozen.

Gross sales=sum original subtotal before discounts/tax. Sales after discount=gross
minus receipt discount. Net sales=sales after discount minus refund net, excluding
tax. Net tax=sale tax minus refund tax. Value after refunds=charged original totals
minus refund total(net+tax). Charged sale value excludes tendered change; this report
does not claim points are a gateway cash transfer. Sold/returned/net unit quantities
remain separate. Average transaction=charged sales/sale count rounded whole Rupiah,
null when sale count0. Refund-only periods and net units may be negative.

Line allocations reuse the refund engine's largest-remainder discount/tax rule and
canonical item-ID ties, with original quantity budgets. Existing committed refund
item net/tax amounts are used verbatim. Headers/items/refunds are read separately
inside one repeatable-read read-only transaction, preventing join fan-out.
Checked big integers protect aggregate bounds +/-9007199254740991.

Legacy cancelled/refunded receipts without verifiable ledger and inconsistent
original snapshots are excluded with affected receipt counts. Other known totals
remain visible; old values are not repaired/invented. Current-period refund records
with unverifiable parent/line evidence produce an explicit unsupported error.

## Historical category snapshots

Migration000013 adds nullable transaction_items.category_id_snapshot and
category_name_snapshot with a both-null-or-present/nonempty check. No guessed
backfill and no FK that makes historical identity depend on mutable category rows.
New checkout captures them after existing product locks; category FOR SHARE locks
serialize names/activation through receipt/stock/draft commit. Original monetary
snapshots and refund item IDs remain unchanged. Down refuses to erase populated
category evidence. Preexisting items remain grouped as Kategori historis tidak
tercatat. Products that span category changes/unknown history show Beragam in the
product list; category grouping still follows each original line. Product names
use the latest original timestamp/item tie-break; original price min/max handles
price changes. Inactive products remain in history; historical SKU/COGS is not guessed.

## Export, UI and tests

XLSX uses literal inline text, escaped XML and numeric integer cells; product names
cannot become formulas. Workbook includes report/date/search/sort/definitions/WIB/
generation time and totals. Flutter uses the current auth client, account/request
response guards, Figma local assets, responsive tables and independently scaled
transaction/Rupiah chart series. Browsing/back/export never mutates the cashier cart.
Mock mode reads the same existing memory-only sale/refund collections and captures
new mock sale categories, without copying inventory. Mock workbook shares literal
ZIP/XML writer conventions with catalog export.

Run go test -p1 ./... (single Windows build job avoids linker resource pressure),
go generate ./internal/server and guarded loopback DIKASA_TEST_DATABASE_URL tests
for reports/payment/catalog/refund/cash/savedorder/migrations. Fixtures use temporary
schemas, never application sale/refund/session mutations. Flutter has report service/
state/widget/mock tests and optional --dart-define=CAPTURE_REPORTS=true renders.
See the workspace dated report implementation log for actual counts and migration.
