package sales

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"be-dikasa/internal/user"
)

const (
	defaultPageSize = 30
	maximumPageSize = 100
)

// AccountFinder mengambil scope outlet user terautentikasi.
type AccountFinder interface {
	FindByIDWithRole(context.Context, string) (*user.User, error)
}

// Service menyediakan daftar order dan pelanggan dalam scope outlet user.
type Service struct {
	repository Repository
	accounts   AccountFinder
}

// NewService membuat service daftar penjualan.
func NewService(repository Repository, accounts AccountFinder) *Service {
	return &Service{repository: repository, accounts: accounts}
}

func (service *Service) outletID(ctx context.Context, userID string) (string, error) {
	account, err := service.accounts.FindByIDWithRole(ctx, userID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return "", ErrUserNotFound
		}
		return "", err
	}
	if account.OutletID == nil || strings.TrimSpace(*account.OutletID) == "" {
		return "", ErrForbidden
	}
	return strings.TrimSpace(*account.OutletID), nil
}

// ListOrders mengambil halaman order outlet setelah memvalidasi filter.
func (service *Service) ListOrders(ctx context.Context, userID string, filter OrderFilter) (OrderPage, error) {
	outletID, err := service.outletID(ctx, userID)
	if err != nil {
		return OrderPage{}, err
	}
	filter.Search = strings.TrimSpace(filter.Search)
	filter.OrderTypeID = strings.TrimSpace(filter.OrderTypeID)
	filter.Status = strings.TrimSpace(filter.Status)
	if filter.OrderTypeID != "" {
		if _, parseErr := uuid.Parse(filter.OrderTypeID); parseErr != nil {
			return OrderPage{}, ErrValidation
		}
	}
	if filter.Status != "" && !validOrderStatus(filter.Status) {
		return OrderPage{}, ErrValidation
	}
	if err = normalizePagination(&filter.Page, &filter.PageSize); err != nil {
		return OrderPage{}, err
	}
	filter.OutletID = outletID
	values, total, err := service.repository.ListOrders(ctx, filter)
	if err != nil {
		return OrderPage{}, err
	}
	return OrderPage{Data: values, Page: filter.Page, PageSize: filter.PageSize, Total: total}, nil
}

// ListCustomers mengambil halaman pelanggan aktif outlet setelah memvalidasi filter.
func (service *Service) ListCustomers(ctx context.Context, userID string, filter CustomerFilter) (CustomerPage, error) {
	outletID, err := service.outletID(ctx, userID)
	if err != nil {
		return CustomerPage{}, err
	}
	filter.Search = strings.TrimSpace(filter.Search)
	filter.GenderID = strings.TrimSpace(filter.GenderID)
	if filter.GenderID != "" {
		if _, parseErr := uuid.Parse(filter.GenderID); parseErr != nil {
			return CustomerPage{}, ErrValidation
		}
	}
	if err = normalizePagination(&filter.Page, &filter.PageSize); err != nil {
		return CustomerPage{}, err
	}
	filter.OutletID = outletID
	values, total, err := service.repository.ListCustomers(ctx, filter)
	if err != nil {
		return CustomerPage{}, err
	}
	return CustomerPage{Data: values, Page: filter.Page, PageSize: filter.PageSize, Total: total}, nil
}

func normalizePagination(page, pageSize *int) error {
	if *page < 1 {
		*page = 1
	}
	if *pageSize < 1 {
		*pageSize = defaultPageSize
	}
	if *pageSize > maximumPageSize {
		return ErrValidation
	}
	return nil
}

func validOrderStatus(status string) bool {
	switch status {
	case StatusPaid, StatusPartiallyRefunded, StatusRefunded, StatusCancelled:
		return true
	default:
		return false
	}
}
