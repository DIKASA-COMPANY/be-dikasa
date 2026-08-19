package cash

import (
	"context"
	"errors"
	"testing"
	"time"

	"be-dikasa/internal/user"
)

type stubRepository struct {
	created *Session
	current *Session
	err     error
}

func (repository *stubRepository) Create(_ context.Context, session *Session) error {
	repository.created = session
	return repository.err
}

func (repository *stubRepository) FindOpenByOutletID(context.Context, string) (*Session, error) {
	return repository.current, repository.err
}

type stubUserFinder struct {
	account *user.User
	err     error
}

func (finder stubUserFinder) FindByID(context.Context, string) (*user.User, error) {
	return finder.account, finder.err
}

func TestServiceOpensCashSessionForAuthenticatedUsersOutlet(t *testing.T) {
	openedAt := time.Date(2026, time.August, 19, 8, 30, 0, 0, time.FixedZone("WIB", 7*60*60))
	outletID := "outlet-id"
	repository := &stubRepository{}
	service := NewService(repository, stubUserFinder{account: &user.User{ID: "user-id", OutletID: &outletID}}, func() time.Time { return openedAt })

	session, err := service.Open(context.Background(), "user-id", 500_000)

	if err != nil {
		t.Fatalf("open cash session: %v", err)
	}
	if repository.created != session {
		t.Fatal("service must persist and return the same session")
	}
	if session.UserID != "user-id" || session.OutletID != outletID || session.OpeningCash != 500_000 {
		t.Fatalf("unexpected cash session: %+v", session)
	}
	if !session.OpenedAt.Equal(openedAt) || session.Status != StatusOpen {
		t.Fatalf("unexpected opening state: %+v", session)
	}
}

func TestServiceRejectsOpeningCashWithoutOutlet(t *testing.T) {
	repository := &stubRepository{}
	service := NewService(repository, stubUserFinder{account: &user.User{ID: "user-id"}}, func() time.Time { return time.Time{} })

	_, err := service.Open(context.Background(), "user-id", 500_000)

	if !errors.Is(err, ErrUserWithoutOutlet) {
		t.Fatalf("expected ErrUserWithoutOutlet, got %v", err)
	}
	if repository.created != nil {
		t.Fatal("session must not be persisted without an outlet")
	}
}

func TestServiceRejectsNegativeOpeningCash(t *testing.T) {
	outletID := "outlet-id"
	repository := &stubRepository{}
	service := NewService(repository, stubUserFinder{account: &user.User{ID: "user-id", OutletID: &outletID}}, func() time.Time { return time.Time{} })

	_, err := service.Open(context.Background(), "user-id", -1)

	if !errors.Is(err, ErrInvalidOpeningCash) {
		t.Fatalf("expected ErrInvalidOpeningCash, got %v", err)
	}
	if repository.created != nil {
		t.Fatal("negative opening cash must not be persisted")
	}
}

func TestServiceRejectsOpeningCashAboveJSONSafeRupiahLimit(t *testing.T) {
	outletID := "outlet-id"
	repository := &stubRepository{}
	service := NewService(repository, stubUserFinder{account: &user.User{ID: "user-id", OutletID: &outletID}}, func() time.Time { return time.Time{} })

	_, err := service.Open(context.Background(), "user-id", 9_007_199_254_740_992)

	if !errors.Is(err, ErrInvalidOpeningCash) {
		t.Fatalf("expected ErrInvalidOpeningCash, got %v", err)
	}
	if repository.created != nil {
		t.Fatal("opening cash above the supported limit must not be persisted")
	}
}

func TestServiceGetsCurrentCashSessionForUsersOutlet(t *testing.T) {
	outletID := "outlet-id"
	expected := &Session{ID: "session-id", OutletID: outletID, UserID: "cashier-id", OpeningCash: 500_000, Status: StatusOpen}
	service := NewService(&stubRepository{current: expected}, stubUserFinder{account: &user.User{ID: "user-id", OutletID: &outletID}}, func() time.Time { return time.Time{} })

	actual, err := service.Current(context.Background(), "user-id")

	if err != nil {
		t.Fatalf("get current cash session: %v", err)
	}
	if actual != expected {
		t.Fatalf("expected %+v, got %+v", expected, actual)
	}
}
