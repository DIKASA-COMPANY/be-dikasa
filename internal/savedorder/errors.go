package savedorder

import "errors"

// ErrValidation indicates an invalid cart or reference.
var ErrValidation = errors.New("invalid saved order")

// ErrNotFound hides missing and cross-outlet orders alike.
var ErrNotFound = errors.New("saved order not found")

// ErrForbidden rejects accounts without a cashier/manager outlet role.
var ErrForbidden = errors.New("saved order forbidden")

// ErrConflict indicates a stale or closed draft.
var ErrConflict = errors.New("saved order revision conflict")

// ErrRequestUsed prevents reusing a command UUID with a different payload.
var ErrRequestUsed = errors.New("saved order request id reused")

// ErrAlreadyPaid prevents a second receipt for an already completed draft.
var ErrAlreadyPaid = errors.New("saved order already paid")
