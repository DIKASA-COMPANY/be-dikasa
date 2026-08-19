package reservation

import (
	"context"
	"errors"
	"testing"
	"time"

	"be-dikasa/internal/user"
)

type fakeRepository struct {
	typeValue  *Type
	tableValue *Table
	created    *Reservation
	conflict   bool
}

func (repository *fakeRepository) ListTypes(context.Context) ([]Type, error) { return nil, nil }
func (repository *fakeRepository) ListTables(context.Context, string, *time.Time) ([]Table, error) {
	return nil, nil
}
func (repository *fakeRepository) FindType(context.Context, string) (*Type, error) {
	return repository.typeValue, nil
}
func (repository *fakeRepository) FindTable(context.Context, string, string) (*Table, error) {
	return repository.tableValue, nil
}
func (repository *fakeRepository) CreateTable(context.Context, *Table) error             { return nil }
func (repository *fakeRepository) UpdateTable(context.Context, *Table) error             { return nil }
func (repository *fakeRepository) SoftDeleteTable(context.Context, string, string) error { return nil }
func (repository *fakeRepository) HasScheduleConflict(context.Context, string, string, time.Time, string) (bool, error) {
	return repository.conflict, nil
}
func (repository *fakeRepository) List(context.Context, Filter) ([]Reservation, int64, error) {
	return nil, 0, nil
}
func (repository *fakeRepository) Find(context.Context, string, string) (*Reservation, error) {
	return repository.created, nil
}
func (repository *fakeRepository) Create(_ context.Context, value *Reservation) error {
	repository.created = value
	return nil
}
func (repository *fakeRepository) Update(_ context.Context, value *Reservation) error {
	repository.created = value
	return nil
}
func (repository *fakeRepository) Cancel(context.Context, string, string) error { return nil }

type fakeAccounts struct{ account *user.User }

func (finder fakeAccounts) FindByIDWithRole(context.Context, string) (*user.User, error) {
	return finder.account, nil
}

func reservationAccount(outletID *string) *user.User {
	return &user.User{ID: "user-a", OutletID: outletID, Role: user.Role{Name: "cashier"}}
}

func TestCreateReservationScopesOutletAndStoresFormOptions(t *testing.T) {
	outletID := "outlet-a"
	reservedAt := time.Date(2026, time.August, 21, 19, 0, 0, 0, time.UTC)
	repository := &fakeRepository{
		typeValue:  &Type{ID: "type-a", Code: "regular", Name: "Reguler", IsActive: true},
		tableValue: &Table{ID: "table-a", OutletID: outletID, Name: "Meja 1", IsActive: true},
	}
	service := NewService(repository, fakeAccounts{account: reservationAccount(&outletID)}, func() time.Time {
		return time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC)
	})

	created, err := service.Create(context.Background(), "user-a", Mutation{
		CustomerName: "  Sari  ", CustomerContact: " 081234567890 ", ReservationTypeID: "type-a",
		RestaurantTableID: "table-a", ReservedAt: reservedAt, Notes: " dekat jendela ",
		PrintReservation: true, SendReservation: true,
	})
	if err != nil {
		t.Fatalf("create reservation: %v", err)
	}
	if created.OutletID != outletID || created.CreatedBy != "user-a" || created.CustomerName != "Sari" || created.Notes != "dekat jendela" || !created.PrintReservation || !created.SendReservation {
		t.Fatalf("unexpected reservation: %+v", created)
	}
}

func TestCreateReservationRejectsPastTimeAndTableConflict(t *testing.T) {
	outletID := "outlet-a"
	repository := &fakeRepository{typeValue: &Type{ID: "type-a", IsActive: true}, tableValue: &Table{ID: "table-a", OutletID: outletID, IsActive: true}}
	service := NewService(repository, fakeAccounts{account: reservationAccount(&outletID)}, func() time.Time {
		return time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC)
	})
	mutation := Mutation{CustomerName: "Sari", CustomerContact: "081234567890", ReservationTypeID: "type-a", RestaurantTableID: "table-a", ReservedAt: time.Date(2026, time.August, 20, 9, 0, 0, 0, time.UTC)}
	if _, err := service.Create(context.Background(), "user-a", mutation); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected past-time validation, got %v", err)
	}
	repository.conflict = true
	mutation.ReservedAt = time.Date(2026, time.August, 21, 9, 0, 0, 0, time.UTC)
	if _, err := service.Create(context.Background(), "user-a", mutation); !errors.Is(err, ErrScheduleConflict) {
		t.Fatalf("expected schedule conflict, got %v", err)
	}
}

func TestReservationRequiresOutlet(t *testing.T) {
	service := NewService(&fakeRepository{}, fakeAccounts{account: reservationAccount(nil)}, time.Now)
	if _, err := service.ListTypes(context.Background(), "user-a"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected outlet requirement, got %v", err)
	}
}
