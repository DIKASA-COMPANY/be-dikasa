package payment

import (
	"context"
	"errors"
	"testing"

	"be-dikasa/internal/user"
)

type stubRepository struct {
	taxPercentage float64
	outletID      string
	err           error
}

func (repository *stubRepository) FindTaxPercentageByOutletID(_ context.Context, outletID string) (float64, error) {
	repository.outletID = outletID
	return repository.taxPercentage, repository.err
}
func (repository *stubRepository) ListWallets(context.Context, string, bool) ([]Wallet, error) {
	return nil, repository.err
}
func (repository *stubRepository) FindWallet(context.Context, string, string, bool) (*Wallet, error) {
	return nil, repository.err
}
func (repository *stubRepository) CreateWallet(context.Context, *Wallet) error {
	return repository.err
}
func (repository *stubRepository) UpdateWallet(context.Context, *Wallet) error {
	return repository.err
}
func (repository *stubRepository) SoftDeleteWallet(context.Context, string, string) error {
	return repository.err
}
func (repository *stubRepository) FindOpenCashSessionIDByOutletID(context.Context, string) (string, error) {
	return "", repository.err
}
func (repository *stubRepository) CreateTransaction(context.Context, *Transaction) error {
	return repository.err
}

type stubUserFinder struct {
	account *user.User
	err     error
}

func (finder stubUserFinder) FindByIDWithRole(context.Context, string) (*user.User, error) {
	return finder.account, finder.err
}

func TestServiceReturnsCurrentOutletTaxPercentage(t *testing.T) {
	outletID := "outlet-id"
	repository := &stubRepository{taxPercentage: 11.5}
	service := NewService(repository, stubUserFinder{account: &user.User{ID: "user-id", OutletID: &outletID}})

	taxPercentage, err := service.TaxPercentage(context.Background(), "user-id")

	if err != nil {
		t.Fatalf("get tax percentage: %v", err)
	}
	if taxPercentage != 11.5 {
		t.Fatalf("expected tax percentage 11.5, got %v", taxPercentage)
	}
	if repository.outletID != outletID {
		t.Fatalf("expected outlet %q, got %q", outletID, repository.outletID)
	}
}

func TestServiceRejectsUserWithoutOutlet(t *testing.T) {
	repository := &stubRepository{}
	service := NewService(repository, stubUserFinder{account: &user.User{ID: "user-id"}})

	_, err := service.TaxPercentage(context.Background(), "user-id")

	if !errors.Is(err, ErrUserWithoutOutlet) {
		t.Fatalf("expected ErrUserWithoutOutlet, got %v", err)
	}
	if repository.outletID != "" {
		t.Fatal("repository must not be called when the user has no outlet")
	}
}

func TestServiceMapsMissingUser(t *testing.T) {
	service := NewService(&stubRepository{}, stubUserFinder{err: user.ErrNotFound})

	_, err := service.TaxPercentage(context.Background(), "missing-user")

	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}
