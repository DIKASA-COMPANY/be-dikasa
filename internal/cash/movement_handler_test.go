package cash

import (
	shared "be-dikasa/internal/shared/errors"
	"context"
	"net/http"
	"testing"
)

func TestMovementHandlerRequiresIdentity(t *testing.T) {
	h := NewMovementHandler(nil)
	_, e := h.Activity(context.Background(), &MovementActivityInput{})
	v, ok := e.(*shared.StatusError)
	if !ok || v.GetStatus() != http.StatusUnauthorized {
		t.Fatal(e)
	}
}
func TestMovementErrorsDoNotLeakStorage(t *testing.T) {
	for _, c := range []struct {
		e      error
		status int
	}{{ErrMovementInvalid, 422}, {ErrMovementInsufficient, 409}, {ErrMovementReversed, 409}, {ErrMovementRequestUsed, 409}, {ErrCloseForbidden, 403}, {ErrProofDisabled, 503}, {ErrProofInvalid, 422}, {ErrProofTooLarge, 413}, {ErrProofStorage, 502}} {
		e := movementError(c.e).(*shared.StatusError)
		if e.GetStatus() != c.status {
			t.Fatal(c, e)
		}
	}
}
