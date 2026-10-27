package sales

import (
	"context"
	"errors"
	"net/http"

	sharederrors "be-dikasa/internal/shared/errors"
	"be-dikasa/internal/user"
)

// Provider mendefinisikan use case daftar order dan pelanggan untuk HTTP.
type Provider interface {
	ListOrders(context.Context, string, OrderFilter) (OrderPage, error)
	ListCustomers(context.Context, string, CustomerFilter) (CustomerPage, error)
}

// Handler melayani endpoint daftar order dan pelanggan.
type Handler struct {
	provider Provider
}

// NewHandler membuat handler daftar penjualan dengan provider yang diberikan.
func NewHandler(provider Provider) *Handler {
	return &Handler{provider: provider}
}

// ListOrders mengembalikan order milik outlet user terautentikasi.
func (handler *Handler) ListOrders(ctx context.Context, input *ListOrdersInput) (*OrdersOutput, error) {
	userID, ok := user.AuthenticatedUserIDFromContext(ctx)
	if !ok {
		return nil, sharederrors.Standard(http.StatusUnauthorized, nil)
	}
	page, err := handler.provider.ListOrders(ctx, userID, OrderFilter{
		Search: input.Search, OrderTypeID: input.OrderTypeID, Status: input.Status, Page: input.Page, PageSize: input.PageSize,
	})
	if err != nil {
		return nil, salesStatusError(err)
	}
	output := &OrdersOutput{}
	output.Body.Data = make([]OrderData, 0, len(page.Data))
	for _, value := range page.Data {
		output.Body.Data = append(output.Body.Data, orderData(value))
	}
	output.Body.Meta = PaginationMeta{Page: page.Page, PageSize: page.PageSize, Total: page.Total}
	return output, nil
}

// ListCustomers mengembalikan pelanggan aktif milik outlet user terautentikasi.
func (handler *Handler) ListCustomers(ctx context.Context, input *ListCustomersInput) (*CustomersOutput, error) {
	userID, ok := user.AuthenticatedUserIDFromContext(ctx)
	if !ok {
		return nil, sharederrors.Standard(http.StatusUnauthorized, nil)
	}
	page, err := handler.provider.ListCustomers(ctx, userID, CustomerFilter{
		Search: input.Search, GenderID: input.GenderID, Page: input.Page, PageSize: input.PageSize,
	})
	if err != nil {
		return nil, salesStatusError(err)
	}
	output := &CustomersOutput{}
	output.Body.Data = make([]CustomerData, 0, len(page.Data))
	for _, value := range page.Data {
		output.Body.Data = append(output.Body.Data, customerData(value))
	}
	output.Body.Meta = PaginationMeta{Page: page.Page, PageSize: page.PageSize, Total: page.Total}
	return output, nil
}

func orderData(value Order) OrderData {
	data := OrderData{
		RefundedAmount: value.RefundedAmount, NetAfterRefund: value.NetAfterRefund, RefundRevision: value.RefundRevision, RefundsSupported: value.RefundsSupported,
		ID: value.ID, ReceiptNumber: value.ReceiptNumber, CustomerID: value.CustomerID, CustomerName: value.CustomerName,
		OrderedAt: value.OrderedAt, OrderType: OrderListTypeData{ID: value.OrderTypeID, Code: value.OrderTypeCode, Name: value.OrderTypeName},
		TotalQuantity: value.TotalQuantity, TotalAmount: value.TotalAmount, Status: value.Status,
	}
	if value.RestaurantTableID != nil && value.RestaurantTableName != nil {
		data.RestaurantTable = &RestaurantTableData{ID: *value.RestaurantTableID, Name: *value.RestaurantTableName}
	}
	return data
}

func customerData(value Customer) CustomerData {
	data := CustomerData{
		ID: value.ID, Name: value.Name, Address: value.Address, Points: value.Points,
		PhoneNumber: value.PhoneNumber, Email: value.Email, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
	if value.GenderID != nil && value.GenderName != nil {
		data.Gender = &GenderData{ID: *value.GenderID, Name: *value.GenderName}
	}
	return data
}

// salesStatusError mencegah detail persistence bocor ke response API.
func salesStatusError(err error) *sharederrors.StatusError {
	switch {
	case errors.Is(err, ErrForbidden):
		return sharederrors.Standard(http.StatusForbidden, nil)
	case errors.Is(err, ErrUserNotFound):
		return sharederrors.Standard(http.StatusNotFound, nil)
	case errors.Is(err, ErrValidation):
		return sharederrors.Standard(http.StatusUnprocessableEntity, nil)
	default:
		return sharederrors.Standard(http.StatusInternalServerError, nil)
	}
}
