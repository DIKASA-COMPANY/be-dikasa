package sales

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	sharederrors "be-dikasa/internal/shared/errors"
	"be-dikasa/internal/user"
)

type stubProvider struct {
	orderPage    OrderPage
	customerPage CustomerPage
	err          error
}

func (provider stubProvider) ListOrders(context.Context, string, OrderFilter) (OrderPage, error) {
	return provider.orderPage, provider.err
}

func (provider stubProvider) ListCustomers(context.Context, string, CustomerFilter) (CustomerPage, error) {
	return provider.customerPage, provider.err
}

func TestHandlerReturnsOrderListForMockup(t *testing.T) {
	tableID, tableName := "table-id", "Meja 5"
	orderedAt := time.Date(2026, time.August, 28, 10, 0, 0, 0, time.UTC)
	handler := NewHandler(stubProvider{orderPage: OrderPage{Data: []Order{{
		ID: "order-id", ReceiptNumber: "5834608256035020", CustomerName: "Jane Doe", OrderedAt: orderedAt,
		OrderTypeID: "type-id", OrderTypeCode: "table_order", OrderTypeName: "Dine-In",
		RestaurantTableID: &tableID, RestaurantTableName: &tableName, TotalQuantity: 7, TotalAmount: 120_000, Status: StatusPaid,
	}}, Page: 1, PageSize: 30, Total: 1}})
	ctx := user.WithAuthenticatedUserID(context.Background(), "user-id")

	output, err := handler.ListOrders(ctx, &ListOrdersInput{Page: 1, PageSize: 30})

	if err != nil {
		t.Fatalf("list orders: %v", err)
	}
	if len(output.Body.Data) != 1 || output.Body.Data[0].ReceiptNumber != "5834608256035020" || output.Body.Data[0].RestaurantTable == nil {
		t.Fatalf("unexpected order data: %+v", output.Body.Data)
	}
	if output.Body.Meta.Total != 1 || output.Body.Meta.Page != 1 {
		t.Fatalf("unexpected order meta: %+v", output.Body.Meta)
	}
}

func TestHandlerReturnsRegisteredCustomerList(t *testing.T) {
	genderID, genderName, email := "gender-id", "female", "jane@example.com"
	handler := NewHandler(stubProvider{customerPage: CustomerPage{Data: []Customer{{
		ID: "customer-id", Name: "Jane Doe", Address: "Jalan Cikawao No. 10, Bandung", Points: 2321,
		GenderID: &genderID, GenderName: &genderName, PhoneNumber: "08123456789", Email: &email,
	}}, Page: 1, PageSize: 30, Total: 1}})
	ctx := user.WithAuthenticatedUserID(context.Background(), "user-id")

	output, err := handler.ListCustomers(ctx, &ListCustomersInput{Page: 1, PageSize: 30})

	if err != nil {
		t.Fatalf("list customers: %v", err)
	}
	if len(output.Body.Data) != 1 || output.Body.Data[0].Name != "Jane Doe" || output.Body.Data[0].Gender == nil || output.Body.Data[0].Points != 2321 {
		t.Fatalf("unexpected customer data: %+v", output.Body.Data)
	}
}

func TestHandlerRequiresAuthenticationForSalesLists(t *testing.T) {
	handler := NewHandler(stubProvider{})

	_, err := handler.ListOrders(context.Background(), &ListOrdersInput{})

	statusError, ok := err.(*sharederrors.StatusError)
	if !ok || statusError.GetStatus() != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized StatusError, got %T %v", err, err)
	}
}

func TestHandlerMapsSalesListErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "user without outlet", err: ErrForbidden, status: http.StatusForbidden},
		{name: "missing user", err: ErrUserNotFound, status: http.StatusNotFound},
		{name: "invalid filter", err: ErrValidation, status: http.StatusUnprocessableEntity},
		{name: "repository failure", err: errors.New("database unavailable"), status: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := NewHandler(stubProvider{err: test.err})
			ctx := user.WithAuthenticatedUserID(context.Background(), "user-id")

			_, err := handler.ListCustomers(ctx, &ListCustomersInput{})

			statusError, ok := err.(*sharederrors.StatusError)
			if !ok || statusError.GetStatus() != test.status {
				t.Fatalf("expected status %d, got %T %v", test.status, err, err)
			}
		})
	}
}
