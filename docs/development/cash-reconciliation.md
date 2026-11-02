# Cash-session reconciliation

Implemented POS-07 using Supabase Auth, Supabase Storage, and PostgreSQL.

## Cash calculation and audit

One shared till is open per outlet. Expected physical cash is opening_cash plus
original cash receipt paid_amount-change_amount minus refunds whose OWN method is
cash and whose cash_session_id names this session. Receipt status changes do not
erase original inflow. Noncash totals remain a separate method breakdown. A refund
of an older receipt affects the paying session; the previous closed summary never
changes. Manual cash-in/out is now included through CASH-02; expense accounting, tips workflow and bank settlement remain separate. Existing retained tender values are used; no inferred missing payouts.

Amounts use whole Rupiah. Counted cash can be zero. Difference=counted-expected;
positive is surplus, negative is shortage. Nonzero difference requires a nonempty
reason up to500 Unicode characters. Expected/difference can be negative, bounded
within +/-9007199254740991. Big-integer intermediates prevent overflow. Unsupported
legacy refunded/cancelled receipt evidence or inconsistent amounts block closure;
legacy closed sessions without reconciliation retain null closing summary.

## Contract and recovery

All endpoints below use authenticated /dikasa/v1 and the existing data/error envelope.
Cashiers/managers may read within their outlet. Only the opening cashier or manager
can preview/close. Foreign session IDs return404; other roles/closers return403.

| Method | Endpoint | Body / result |
| --- | --- | --- |
| GET | /cash-sessions?page=1&page_size=20 | data array and meta; page_size max100 |
| GET | /cash-sessions/{session_id} | open summary/can_close or immutable closing; legacy may have no summary |
| POST | /cash-sessions/{session_id}/close/preview | counted_cash, reason; returns summary, difference, preview_hash |
| POST | /cash-sessions/{session_id}/close | counted_cash, reason, request_id UUID, preview_hash; returns200 immutable result |

Existing POST /cash-sessions and GET /cash-sessions/current remain compatible.
OpenAPI is generated from registered Huma DTOs. Bruno cash-sessions contains six new
examples. Choose a fresh UUID for each intended close; reuse the exact original body
on timeout. The close result carries session_id and close_request_id; scoped detail
returns that same closing result after a lost response. An open/notfound read is
not proof the write failed. Never change a pending target to a newly opened session.
Business409 codes: SESI_KAS_PRATINJAU_BERUBAH, SESI_KAS_SUDAH_DITUTUP,
REQUEST_ID_DIGUNAKAN. Validation422 HITUNGAN_KAS_TIDAK_VALID; unsupported422
REKONSILIASI_TIDAK_DIDUKUNG. Reload activity and preview again after stale refusal.

Closure uses an outlet/request advisory lock and an exclusive session row lock.
Same-payload replay returns the original immutable JSON before mutable checks.
Payment creation now checks and locks the exact target open session FOR SHARE
inside its write transaction; refund confirmation already holds the same shared
session lock. Close waits for previous writers and recomputes activity before hash
comparison. Later writers cannot write into the closed session. Original receipts,
refunds, stock and saved drafts are untouched by closing itself.

## Schema and verification

Additive migration000012 adds nullable close identity/name, reason, signed expected/
difference, request UUID/hash, preview hash and JSON response on cash_sessions.
All new metadata must be present together for reconciled closed rows; all-null
metadata preserves legacy rows. Scoped unique UUID index protects replay. Down
refuses to erase saved reconciliations; do not use it as an application undo action.

Run go test ./..., go generate ./internal/server and opt-in PostgreSQL tests with a
privately loaded loopback DIKASA_TEST_DATABASE_URL:
`go test ./internal/cash ./internal/payment ./internal/refund ./internal/savedorder ./migrations -count=1`.
Fixtures create/drop only random temporary schemas. Do not close a real user's till
for verification. Flutter has service/controller/widget/mock/POS-lock tests plus
optional --dart-define=CAPTURE_CASH=true for real-font layout PNGs.

Mock sessions share existing mock receipts/refunds and inventory; they persist only
in memory. Demo hash is deterministic; production preview/command hashes use SHA-256.
No reopening/editing closed summaries, automatic carry-forward, general sales
reports, hardware integration, export/printing or customer management in this feature.


## Manual drawer movements (CASH-02)

Expected cash adds cash_in and subtracts cash_out, including linked correction
entries. Only the opener or manager can add entries; opener reverses own entries,
manager any original, once and only while open. Outflows cannot overdraw expected
cash. Totals/hash include every permanent event; a zero-net correction still stales
an earlier preview. Historical frozen JSON is preserved; absent movement totals=0.

Use the cash-movements Bruno folder and generated OpenAPI for activity, proof upload
intent, multipart create without file bytes, detail, actor-bound recovery, reversal and proof-URL routes.
All are under /dikasa/v1/cash-sessions/{session_id}. Exact UUID/body replay returns
original success even after closure. A missing lookup is not proof failure.

Cash proof evidence uses a signed Supabase upload URL bound to outlet, session, actor,
and movement request UUID under
`outlets/{outlet_id}/cash-sessions/{session_id}/users/{actor_id}/requests/{request_id}/proofs/{uuid}.{ext}`.
The client uploads bytes directly, then sends `proof_object_name` with the same request UUID.
The backend revalidates the stored object before PostgreSQL stores only `proof_url`.
The authenticated same-outlet proof endpoint returns that public URL; no image bytes pass
through the DIKASA API. Three MiB, 4096 px/16 MP limits apply. Invalid or failed-adoption
objects are deleted; scheduled storage reconciliation removes expired unreferenced intents.
