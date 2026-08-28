package sales

import (
	"context"
	"errors"
	"testing"

	"be-dikasa/internal/user"
)

type stubRepository struct {
	orders         []Order
	customers      []Customer
	total          int64
	orderFilter    OrderFilter
	customerFilter CustomerFilter
	err            error
}

func (repository *stubRepository) ListOrders(_ context.Context, filter OrderFilter) ([]Order, int64, error) {
	repository.orderFilter = filter
	return repository.orders, repository.total, repository.err
}

func (repository *stubRepository) ListCustomers(_ context.Context, filter CustomerFilter) ([]Customer, int64, error) {
	repository.customerFilter = filter
	return repository.customers, repository.total, repository.err
}

type stubAccountFinder struct {
	account *user.User
	err     error
}

func (finder stubAccountFinder) FindByIDWithRole(context.Context, string) (*user.User, error) {
	return finder.account, finder.err
}

func TestServiceListsOutletOrdersWithNormalizedPagination(t *testing.T) {
	outletID := "outlet-id"
	repository := &stubRepository{orders: []Order{{ID: "order-id"}}, total: 1}
	service := NewService(repository, stubAccountFinder{account: &user.User{OutletID: &outletID}})

	page, err := service.ListOrders(context.Background(), "user-id", OrderFilter{Search: "  Jane  ", Status: StatusPaid})

	if err != nil {
		t.Fatalf("list orders: %v", err)
	}
	if repository.orderFilter.OutletID != outletID || repository.orderFilter.Search != "Jane" {
		t.Fatalf("unexpected repository filter: %+v", repository.orderFilter)
	}
	if repository.orderFilter.Page != 1 || repository.orderFilter.PageSize != 30 {
		t.Fatalf("pagination was not normalized: %+v", repository.orderFilter)
	}
	if page.Page != 1 || page.PageSize != 30 || page.Total != 1 || len(page.Data) != 1 {
		t.Fatalf("unexpected order page: %+v", page)
	}
}

func TestServiceListsOutletCustomersWithGenderFilter(t *testing.T) {
	outletID := "outlet-id"
	genderID := "11111111-1111-4111-8111-111111111111"
	repository := &stubRepository{customers: []Customer{{ID: "customer-id"}}, total: 1}
	service := NewService(repository, stubAccountFinder{account: &user.User{OutletID: &outletID}})

	page, err := service.ListCustomers(context.Background(), "user-id", CustomerFilter{Search: "  0812  ", GenderID: genderID, Page: 2, PageSize: 10})

	if err != nil {
		t.Fatalf("list customers: %v", err)
	}
	if repository.customerFilter.OutletID != outletID || repository.customerFilter.Search != "0812" || repository.customerFilter.GenderID != genderID {
		t.Fatalf("unexpected repository filter: %+v", repository.customerFilter)
	}
	if page.Page != 2 || page.PageSize != 10 || page.Total != 1 {
		t.Fatalf("unexpected customer page: %+v", page)
	}
}

func TestServiceRejectsInvalidListFilters(t *testing.T) {
	outletID := "outlet-id"
	tests := []struct {
		name string
		run  func(*Service) error
	}{
		{name: "invalid order status", run: func(service *Service) error {
			_, err := service.ListOrders(context.Background(), "user-id", OrderFilter{Status: "unknown"})
			return err
		}},
		{name: "oversized order page", run: func(service *Service) error {
			_, err := service.ListOrders(context.Background(), "user-id", OrderFilter{PageSize: 101})
			return err
		}},
		{name: "invalid gender UUID", run: func(service *Service) error {
			_, err := service.ListCustomers(context.Background(), "user-id", CustomerFilter{GenderID: "not-a-uuid"})
			return err
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &stubRepository{}
			service := NewService(repository, stubAccountFinder{account: &user.User{OutletID: &outletID}})

			if err := test.run(service); !errors.Is(err, ErrValidation) {
				t.Fatalf("expected ErrValidation, got %v", err)
			}
			if repository.orderFilter.OutletID != "" || repository.customerFilter.OutletID != "" {
				t.Fatal("repository must not be called for invalid filters")
			}
		})
	}
}

func TestServiceRejectsUserWithoutOutlet(t *testing.T) {
	service := NewService(&stubRepository{}, stubAccountFinder{account: &user.User{}})

	_, err := service.ListOrders(context.Background(), "user-id", OrderFilter{})

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestServiceMapsMissingUser(t *testing.T) {
	service := NewService(&stubRepository{}, stubAccountFinder{err: user.ErrNotFound})

	_, err := service.ListCustomers(context.Background(), "missing-user", CustomerFilter{})

	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}
