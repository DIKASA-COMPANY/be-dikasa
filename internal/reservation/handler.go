package reservation

import (
	"context"
	"errors"
	"net/http"
	"time"

	sharederrors "be-dikasa/internal/shared/errors"
	sharedvalidator "be-dikasa/internal/shared/validator"
	"be-dikasa/internal/user"
)

// Provider mendefinisikan use case reservasi untuk HTTP.
type Provider interface {
	ListTypes(context.Context, string) ([]Type, error)
	ListTables(context.Context, string, *time.Time) ([]Table, error)
	CreateTable(context.Context, string, TableMutation) (*Table, error)
	UpdateTable(context.Context, string, string, TableMutation) (*Table, error)
	DeleteTable(context.Context, string, string) error
	List(context.Context, string, Filter) ([]Reservation, int64, error)
	Get(context.Context, string, string) (*Reservation, error)
	Create(context.Context, string, Mutation) (*Reservation, error)
	Update(context.Context, string, string, Mutation) (*Reservation, error)
	Cancel(context.Context, string, string) error
}

// Handler melayani endpoint reservasi.
type Handler struct {
	provider  Provider
	validator *sharedvalidator.Validator
}

// NewHandler membuat handler reservasi.
func NewHandler(provider Provider, validator *sharedvalidator.Validator) *Handler {
	return &Handler{provider: provider, validator: validator}
}

func authenticated(ctx context.Context) (string, error) {
	value, ok := user.AuthenticatedUserIDFromContext(ctx)
	if !ok {
		return "", sharederrors.Standard(http.StatusUnauthorized, nil)
	}
	return value, nil
}

// ListTypes melayani dropdown jenis reservasi.
func (handler *Handler) ListTypes(ctx context.Context, _ *struct{}) (*TypesOutput, error) {
	userID, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	values, err := handler.provider.ListTypes(ctx, userID)
	if err != nil {
		return nil, statusError(err)
	}
	output := &TypesOutput{}
	output.Body.Data = make([]TypeData, 0, len(values))
	for _, value := range values {
		output.Body.Data = append(output.Body.Data, typeData(value))
	}
	return output, nil
}

// ListTables melayani dropdown meja aktif/tersedia.
func (handler *Handler) ListTables(ctx context.Context, input *ListTablesInput) (*TablesOutput, error) {
	userID, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	var reservedAt *time.Time
	if input.ReservationAt != "" {
		parsed, parseErr := time.Parse(time.RFC3339, input.ReservationAt)
		if parseErr != nil {
			return nil, statusError(ErrValidation)
		}
		reservedAt = &parsed
	}
	values, err := handler.provider.ListTables(ctx, userID, reservedAt)
	if err != nil {
		return nil, statusError(err)
	}
	output := &TablesOutput{}
	output.Body.Data = make([]TableData, 0, len(values))
	for _, value := range values {
		output.Body.Data = append(output.Body.Data, tableData(value))
	}
	return output, nil
}

// CreateTable melayani pembuatan meja oleh manager.
func (handler *Handler) CreateTable(ctx context.Context, input *TableCreateInput) (*TableOutput, error) {
	userID, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	if details := handler.validator.Validate(input.Body); len(details) > 0 {
		return nil, validationError(details)
	}
	value, err := handler.provider.CreateTable(ctx, userID, tableMutation(input.Body, true))
	if err != nil {
		return nil, statusError(err)
	}
	output := &TableOutput{Status: http.StatusCreated}
	output.Body.Data = tableData(*value)
	return output, nil
}

// UpdateTable melayani perubahan meja oleh manager.
func (handler *Handler) UpdateTable(ctx context.Context, input *TableMutationInput) (*TableOutput, error) {
	userID, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	if details := handler.validator.Validate(input.Body); len(details) > 0 {
		return nil, validationError(details)
	}
	value, err := handler.provider.UpdateTable(ctx, userID, input.TableID, tableMutation(input.Body, input.Body.IsActive))
	if err != nil {
		return nil, statusError(err)
	}
	output := &TableOutput{}
	output.Body.Data = tableData(*value)
	return output, nil
}

// DeleteTable melayani soft delete meja oleh manager.
func (handler *Handler) DeleteTable(ctx context.Context, input *TableDeleteInput) (*NoContentOutput, error) {
	userID, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	if err = handler.provider.DeleteTable(ctx, userID, input.TableID); err != nil {
		return nil, statusError(err)
	}
	return &NoContentOutput{Status: http.StatusNoContent}, nil
}

// List melayani daftar reservasi outlet.
func (handler *Handler) List(ctx context.Context, input *ListReservationsInput) (*ReservationsOutput, error) {
	userID, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	filter := Filter{Date: input.Date, Status: input.Status, Page: input.Page, PageSize: input.PageSize}
	values, total, err := handler.provider.List(ctx, userID, filter)
	if err != nil {
		return nil, statusError(err)
	}
	output := &ReservationsOutput{}
	output.Body.Data = reservationsData(values)
	output.Body.Meta.Page, output.Body.Meta.PageSize, output.Body.Meta.Total = input.Page, input.PageSize, total
	return output, nil
}

// Get melayani detail reservasi.
func (handler *Handler) Get(ctx context.Context, input *ReservationIDInput) (*ReservationOutput, error) {
	userID, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	value, err := handler.provider.Get(ctx, userID, input.ReservationID)
	if err != nil {
		return nil, statusError(err)
	}
	output := &ReservationOutput{}
	output.Body.Data = reservationData(*value)
	return output, nil
}

// Create melayani pembuatan reservasi.
func (handler *Handler) Create(ctx context.Context, input *ReservationCreateInput) (*ReservationOutput, error) {
	userID, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	if details := handler.validator.Validate(input.Body); len(details) > 0 {
		return nil, validationError(details)
	}
	value, err := handler.provider.Create(ctx, userID, Mutation{CustomerName: input.Body.CustomerName, CustomerContact: input.Body.CustomerContact,
		ReservationTypeID: input.Body.ReservationTypeID, RestaurantTableID: input.Body.RestaurantTableID, ReservedAt: input.Body.ReservedAt,
		Notes: input.Body.Notes, PrintReservation: input.Body.PrintReservation, SendReservation: input.Body.SendReservation})
	if err != nil {
		return nil, statusError(err)
	}
	output := &ReservationOutput{Status: http.StatusCreated}
	output.Body.Data = reservationData(*value)
	return output, nil
}

// Update melayani perubahan parsial reservasi.
func (handler *Handler) Update(ctx context.Context, input *ReservationMutationInput) (*ReservationOutput, error) {
	userID, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	mutation := patchMutation(input.Body)
	value, err := handler.provider.Update(ctx, userID, input.ReservationID, mutation)
	if err != nil {
		return nil, statusError(err)
	}
	output := &ReservationOutput{}
	output.Body.Data = reservationData(*value)
	return output, nil
}

// Cancel melayani pembatalan reservasi.
func (handler *Handler) Cancel(ctx context.Context, input *ReservationIDInput) (*NoContentOutput, error) {
	userID, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	if err = handler.provider.Cancel(ctx, userID, input.ReservationID); err != nil {
		return nil, statusError(err)
	}
	return &NoContentOutput{Status: http.StatusNoContent}, nil
}

func patchMutation(body ReservationPatchRequest) Mutation {
	value := Mutation{Fields: map[string]bool{}}
	if body.CustomerName != nil {
		value.CustomerName = *body.CustomerName
		value.Fields["customer_name"] = true
	}
	if body.CustomerContact != nil {
		value.CustomerContact = *body.CustomerContact
		value.Fields["customer_contact"] = true
	}
	if body.ReservationTypeID != nil {
		value.ReservationTypeID = *body.ReservationTypeID
		value.Fields["reservation_type_id"] = true
	}
	if body.RestaurantTableID != nil {
		value.RestaurantTableID = *body.RestaurantTableID
		value.Fields["restaurant_table_id"] = true
	}
	if body.ReservedAt != nil {
		value.ReservedAt = *body.ReservedAt
		value.Fields["reserved_at"] = true
	}
	if body.Notes != nil {
		value.Notes = *body.Notes
		value.Fields["notes"] = true
	}
	if body.PrintReservation != nil {
		value.PrintReservation = *body.PrintReservation
		value.Fields["print_reservation"] = true
	}
	if body.SendReservation != nil {
		value.SendReservation = *body.SendReservation
		value.Fields["send_reservation"] = true
	}
	if body.Status != nil {
		value.Status = *body.Status
		value.Fields["status"] = true
	}
	return value
}

func typeData(value Type) TypeData {
	return TypeData{ID: value.ID, Code: value.Code, Name: value.Name, SortOrder: value.SortOrder}
}
func tableData(value Table) TableData {
	return TableData{ID: value.ID, Name: value.Name, Capacity: value.Capacity, SortOrder: value.SortOrder, IsActive: value.IsActive}
}
func reservationData(value Reservation) ReservationData {
	return ReservationData{ID: value.ID, CustomerName: value.CustomerName, CustomerContact: value.CustomerContact, ReservedAt: value.ReservedAt, Notes: value.Notes, PrintReservation: value.PrintReservation, SendReservation: value.SendReservation, Status: value.Status, ReservationType: typeData(value.ReservationType), RestaurantTable: tableData(value.RestaurantTable), CreatedBy: value.CreatedBy, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}
func reservationsData(values []Reservation) []ReservationData {
	output := make([]ReservationData, 0, len(values))
	for _, value := range values {
		output = append(output, reservationData(value))
	}
	return output
}
func tableMutation(value TableRequest, active bool) TableMutation {
	return TableMutation{Name: value.Name, Capacity: value.Capacity, SortOrder: value.SortOrder, IsActive: active}
}
func validationError(details map[string]string) *sharederrors.StatusError {
	return sharederrors.New(422, "VALIDASI_GAGAL", "Data yang dikirim tidak memenuhi ketentuan.", details)
}
func statusError(err error) *sharederrors.StatusError {
	switch {
	case errors.Is(err, ErrForbidden):
		return sharederrors.Standard(403, nil)
	case errors.Is(err, ErrNotFound):
		return sharederrors.Standard(404, nil)
	case errors.Is(err, ErrScheduleConflict):
		return sharederrors.New(409, "JADWAL_MEJA_BENTROK", "Meja sudah direservasi pada waktu tersebut.", nil)
	case errors.Is(err, ErrConflict):
		return sharederrors.Standard(409, nil)
	case errors.Is(err, ErrValidation):
		return sharederrors.Standard(422, nil)
	default:
		return sharederrors.Standard(500, nil)
	}
}
