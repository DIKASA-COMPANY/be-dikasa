// Package payment menyediakan konfigurasi yang dibutuhkan journey pembayaran.
package payment

import (
	"context"
	"errors"
)

var (
	// ErrOutletNotFound menandakan outlet tidak ada atau sudah tidak aktif.
	ErrOutletNotFound = errors.New("payment outlet not found")
	// ErrUserNotFound menandakan identitas token tidak memiliki user PostgreSQL.
	ErrUserNotFound = errors.New("payment user not found")
	// ErrUserWithoutOutlet menandakan user tidak terhubung ke outlet operasional.
	ErrUserWithoutOutlet = errors.New("payment user is not assigned to an outlet")
	// ErrWalletNotFound menandakan wallet tidak ditemukan pada outlet aktif.
	ErrWalletNotFound = errors.New("payment wallet not found")
	// ErrCashSessionNotFound menandakan outlet belum memiliki sesi kas aktif.
	ErrCashSessionNotFound = errors.New("payment cash session not found")
	// ErrForbidden menandakan operasi hanya tersedia untuk manager.
	ErrForbidden = errors.New("payment access is forbidden")
	// ErrValidation menandakan payload wallet atau transaksi tidak konsisten.
	ErrValidation = errors.New("payment input is invalid")
	// ErrConflict menandakan code, nama, atau nomor transaksi bentrok.
	ErrConflict = errors.New("payment data conflicts")
	// ErrInvalidReference menandakan referensi transaksi tidak berada pada outlet yang sama.
	ErrInvalidReference = errors.New("payment reference is invalid")
)

// Repository mengambil konfigurasi pembayaran milik outlet.
type Repository interface {
	FindTaxPercentageByOutletID(ctx context.Context, outletID string) (float64, error)
	ListWallets(ctx context.Context, outletID string, includeInactive bool) ([]Wallet, error)
	FindWallet(ctx context.Context, outletID, walletID string, includeInactive bool) (*Wallet, error)
	CreateWallet(ctx context.Context, wallet *Wallet) error
	UpdateWallet(ctx context.Context, wallet *Wallet) error
	SoftDeleteWallet(ctx context.Context, outletID, walletID string) error
	FindOpenCashSessionIDByOutletID(ctx context.Context, outletID string) (string, error)
	CreateTransaction(ctx context.Context, transaction *Transaction) error
}
