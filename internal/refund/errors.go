package refund

import "errors"

// ErrValidation rejects malformed selections.
var ErrValidation = errors.New("invalid refund selection")

// ErrNotFound hides inaccessible outlet records.
var ErrNotFound = errors.New("refund receipt not found")

// ErrForbidden restricts refund operations to outlet cashiers/managers.
var ErrForbidden = errors.New("refund forbidden")

// ErrRevision rejects stale confirmation or changed previews.
var ErrRevision = errors.New("refund revision or preview changed")

// ErrQuantity prevents refunding units more than once.
var ErrQuantity = errors.New("refund quantity unavailable")

// ErrRequestUsed rejects changed payloads under an existing request UUID.
var ErrRequestUsed = errors.New("refund request id reused")

// ErrLegacy rejects inconsistent or unsupported receipt history.
var ErrLegacy = errors.New("unsupported receipt snapshot")

// ErrCashSession requires current cash-session attribution.
var ErrCashSession = errors.New("no open refund cash session")

// ErrRestock rejects inventory without original sale evidence or current capacity.
var ErrRestock = errors.New("invalid refund stock return")
