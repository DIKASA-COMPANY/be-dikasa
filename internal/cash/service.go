package cash

import (
	"context"
	"errors"
	"time"

	"be-dikasa/internal/user"
)

const maxOpeningCash = int64(9_007_199_254_740_991)

var (
	// ErrInvalidOpeningCash menandakan nominal kas awal berada di luar aturan domain.
	ErrInvalidOpeningCash = errors.New("opening cash is outside the supported range")
	// ErrUserWithoutOutlet menandakan user tidak terhubung ke outlet operasional.
	ErrUserWithoutOutlet = errors.New("user is not assigned to an outlet")
	// ErrUserNotFound menandakan identitas token tidak memiliki user PostgreSQL.
	ErrUserNotFound = errors.New("cash session user not found")
)

// UserFinder mengambil user beserta outlet operasionalnya.
type UserFinder interface {
	FindByID(ctx context.Context, id string) (*user.User, error)
}

// Clock menghasilkan waktu pembukaan sesi dan dapat diganti pada test.
type Clock func() time.Time

// Service menyediakan use case sesi kas tanpa bergantung pada transport HTTP.
type Service struct {
	repository Repository
	users      UserFinder
	now        Clock
}

// NewService membuat cash service dengan repository, sumber user, dan clock yang diberikan.
func NewService(repository Repository, users UserFinder, now Clock) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repository: repository, users: users, now: now}
}

// Open membuka satu sesi kas untuk outlet user terautentikasi.
func (service *Service) Open(ctx context.Context, userID string, openingCash int64) (*Session, error) {
	if openingCash < 0 || openingCash > maxOpeningCash {
		return nil, ErrInvalidOpeningCash
	}
	account, err := service.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	if account.OutletID == nil || *account.OutletID == "" {
		return nil, ErrUserWithoutOutlet
	}
	session := &Session{
		OutletID: *account.OutletID, UserID: userID, OpeningCash: openingCash,
		Status: StatusOpen, OpenedAt: service.now(),
	}
	if err := service.repository.Create(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

// Current mengambil sesi kas aktif milik outlet user terautentikasi.
func (service *Service) Current(ctx context.Context, userID string) (*Session, error) {
	account, err := service.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	if account.OutletID == nil || *account.OutletID == "" {
		return nil, ErrUserWithoutOutlet
	}
	return service.repository.FindOpenByOutletID(ctx, *account.OutletID)
}
