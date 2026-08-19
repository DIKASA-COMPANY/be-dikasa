package reservation

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"be-dikasa/internal/user"
)

// Service menerapkan aturan reservasi dan isolasi outlet.
type Service struct {
	repository Repository
	accounts   AccountFinder
	now        func() time.Time
}

// NewService membuat service reservasi.
func NewService(repository Repository, accounts AccountFinder, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repository: repository, accounts: accounts, now: now}
}

func (service *Service) scope(ctx context.Context, userID string, managerOnly bool) (string, error) {
	account, err := service.accounts.FindByIDWithRole(ctx, userID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return "", ErrNotFound
		}
		return "", err
	}
	if account.OutletID == nil || strings.TrimSpace(*account.OutletID) == "" {
		return "", ErrForbidden
	}
	if managerOnly && !strings.EqualFold(account.Role.Name, "manager") {
		return "", ErrForbidden
	}
	return *account.OutletID, nil
}

// ListTypes mengambil dropdown jenis reservasi global untuk user outlet.
func (service *Service) ListTypes(ctx context.Context, userID string) ([]Type, error) {
	if _, err := service.scope(ctx, userID, false); err != nil {
		return nil, err
	}
	return service.repository.ListTypes(ctx)
}

// ListTables mengambil meja aktif dan dapat menyaring ketersediaan waktu.
func (service *Service) ListTables(ctx context.Context, userID string, reservedAt *time.Time) ([]Table, error) {
	outletID, err := service.scope(ctx, userID, false)
	if err != nil {
		return nil, err
	}
	return service.repository.ListTables(ctx, outletID, reservedAt)
}

// CreateTable membuat meja baru dan hanya tersedia bagi manager.
func (service *Service) CreateTable(ctx context.Context, userID string, mutation TableMutation) (*Table, error) {
	outletID, err := service.scope(ctx, userID, true)
	if err != nil {
		return nil, err
	}
	mutation.Name = strings.TrimSpace(mutation.Name)
	if mutation.Name == "" || mutation.Capacity < 1 {
		return nil, ErrValidation
	}
	value := &Table{ID: uuid.NewString(), OutletID: outletID, Name: mutation.Name, Capacity: mutation.Capacity, SortOrder: mutation.SortOrder, IsActive: true}
	if err = service.repository.CreateTable(ctx, value); err != nil {
		return nil, err
	}
	return value, nil
}

// UpdateTable memperbarui konfigurasi meja pada outlet manager.
func (service *Service) UpdateTable(ctx context.Context, userID, id string, mutation TableMutation) (*Table, error) {
	outletID, err := service.scope(ctx, userID, true)
	if err != nil {
		return nil, err
	}
	value, err := service.repository.FindTable(ctx, outletID, id)
	if err != nil {
		return nil, err
	}
	mutation.Name = strings.TrimSpace(mutation.Name)
	if mutation.Name == "" || mutation.Capacity < 1 {
		return nil, ErrValidation
	}
	value.Name, value.Capacity, value.SortOrder, value.IsActive = mutation.Name, mutation.Capacity, mutation.SortOrder, mutation.IsActive
	if err = service.repository.UpdateTable(ctx, value); err != nil {
		return nil, err
	}
	return value, nil
}

// DeleteTable menonaktifkan meja tanpa menghapus record.
func (service *Service) DeleteTable(ctx context.Context, userID, id string) error {
	outletID, err := service.scope(ctx, userID, true)
	if err != nil {
		return err
	}
	return service.repository.SoftDeleteTable(ctx, outletID, id)
}

// List mengambil reservasi outlet dengan filter tanggal, status, dan pagination.
func (service *Service) List(ctx context.Context, userID string, filter Filter) ([]Reservation, int64, error) {
	outletID, err := service.scope(ctx, userID, false)
	if err != nil {
		return nil, 0, err
	}
	if filter.Date != "" {
		if _, err = time.Parse("2006-01-02", filter.Date); err != nil {
			return nil, 0, ErrValidation
		}
	}
	if filter.Status != "" && !validStatus(filter.Status) {
		return nil, 0, ErrValidation
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 30
	}
	if filter.PageSize > 100 {
		return nil, 0, ErrValidation
	}
	filter.OutletID = outletID
	return service.repository.List(ctx, filter)
}

// Get mengambil satu reservasi yang dimiliki outlet user.
func (service *Service) Get(ctx context.Context, userID, id string) (*Reservation, error) {
	outletID, err := service.scope(ctx, userID, false)
	if err != nil {
		return nil, err
	}
	return service.repository.Find(ctx, outletID, id)
}

// Create membuat reservasi setelah memvalidasi waktu, referensi, dan konflik meja.
func (service *Service) Create(ctx context.Context, userID string, mutation Mutation) (*Reservation, error) {
	outletID, err := service.scope(ctx, userID, false)
	if err != nil {
		return nil, err
	}
	if err = service.validateMutation(mutation, true); err != nil {
		return nil, err
	}
	reservationType, table, err := service.references(ctx, outletID, mutation.ReservationTypeID, mutation.RestaurantTableID)
	if err != nil {
		return nil, err
	}
	conflict, err := service.repository.HasScheduleConflict(ctx, outletID, table.ID, mutation.ReservedAt, "")
	if err != nil {
		return nil, err
	}
	if conflict {
		return nil, ErrScheduleConflict
	}
	value := &Reservation{ID: uuid.NewString(), OutletID: outletID, ReservationTypeID: reservationType.ID,
		RestaurantTableID: table.ID, CreatedBy: userID, CustomerName: strings.TrimSpace(mutation.CustomerName),
		CustomerContact: strings.TrimSpace(mutation.CustomerContact), ReservedAt: mutation.ReservedAt,
		Notes: strings.TrimSpace(mutation.Notes), PrintReservation: mutation.PrintReservation,
		SendReservation: mutation.SendReservation, Status: StatusBooked, IsActive: true,
		ReservationType: *reservationType, RestaurantTable: *table}
	if err = service.repository.Create(ctx, value); err != nil {
		if errors.Is(err, ErrConflict) {
			return nil, ErrScheduleConflict
		}
		return nil, err
	}
	return value, nil
}

// Update menerapkan PATCH parsial dan memeriksa ulang konflik meja.
func (service *Service) Update(ctx context.Context, userID, id string, mutation Mutation) (*Reservation, error) {
	outletID, err := service.scope(ctx, userID, false)
	if err != nil {
		return nil, err
	}
	value, err := service.repository.Find(ctx, outletID, id)
	if err != nil {
		return nil, err
	}
	isSet := func(field string) bool { return mutation.Fields == nil || mutation.Fields[field] }
	if isSet("customer_name") {
		value.CustomerName = strings.TrimSpace(mutation.CustomerName)
	}
	if isSet("customer_contact") {
		value.CustomerContact = strings.TrimSpace(mutation.CustomerContact)
	}
	if isSet("reserved_at") {
		value.ReservedAt = mutation.ReservedAt
	}
	if isSet("notes") {
		value.Notes = strings.TrimSpace(mutation.Notes)
	}
	if isSet("print_reservation") {
		value.PrintReservation = mutation.PrintReservation
	}
	if isSet("send_reservation") {
		value.SendReservation = mutation.SendReservation
	}
	if isSet("status") {
		value.Status = mutation.Status
	}
	typeID, tableID := value.ReservationTypeID, value.RestaurantTableID
	if isSet("reservation_type_id") {
		typeID = mutation.ReservationTypeID
	}
	if isSet("restaurant_table_id") {
		tableID = mutation.RestaurantTableID
	}
	merged := Mutation{CustomerName: value.CustomerName, CustomerContact: value.CustomerContact, ReservationTypeID: typeID,
		RestaurantTableID: tableID, ReservedAt: value.ReservedAt, Status: value.Status}
	if err = service.validateMutation(merged, isSet("reserved_at")); err != nil {
		return nil, err
	}
	reservationType, table, err := service.references(ctx, outletID, typeID, tableID)
	if err != nil {
		return nil, err
	}
	conflict, err := service.repository.HasScheduleConflict(ctx, outletID, table.ID, value.ReservedAt, value.ID)
	if err != nil {
		return nil, err
	}
	if conflict {
		return nil, ErrScheduleConflict
	}
	value.ReservationTypeID, value.ReservationType = reservationType.ID, *reservationType
	value.RestaurantTableID, value.RestaurantTable = table.ID, *table
	if err = service.repository.Update(ctx, value); err != nil {
		if errors.Is(err, ErrConflict) {
			return nil, ErrScheduleConflict
		}
		return nil, err
	}
	return value, nil
}

// Cancel membatalkan dan menonaktifkan reservasi pada outlet user.
func (service *Service) Cancel(ctx context.Context, userID, id string) error {
	outletID, err := service.scope(ctx, userID, false)
	if err != nil {
		return err
	}
	return service.repository.Cancel(ctx, outletID, id)
}

func (service *Service) validateMutation(mutation Mutation, validateFuture bool) error {
	name, contact := strings.TrimSpace(mutation.CustomerName), strings.TrimSpace(mutation.CustomerContact)
	if len([]rune(name)) < 2 || len([]rune(name)) > 100 || len([]rune(contact)) < 8 || len([]rune(contact)) > 30 || mutation.ReservationTypeID == "" || mutation.RestaurantTableID == "" || mutation.ReservedAt.IsZero() {
		return ErrValidation
	}
	if validateFuture && !mutation.ReservedAt.After(service.now()) {
		return ErrValidation
	}
	if mutation.Status != "" && !validStatus(mutation.Status) {
		return ErrValidation
	}
	return nil
}

func (service *Service) references(ctx context.Context, outletID, typeID, tableID string) (*Type, *Table, error) {
	reservationType, err := service.repository.FindType(ctx, typeID)
	if err != nil {
		return nil, nil, err
	}
	table, err := service.repository.FindTable(ctx, outletID, tableID)
	if err != nil {
		return nil, nil, err
	}
	if !reservationType.IsActive || !table.IsActive || table.OutletID != outletID {
		return nil, nil, ErrNotFound
	}
	return reservationType, table, nil
}

func validStatus(value string) bool {
	switch value {
	case StatusBooked, StatusConfirmed, StatusSeated, StatusCompleted, StatusCancelled:
		return true
	default:
		return false
	}
}
