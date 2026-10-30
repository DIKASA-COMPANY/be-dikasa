package user

import (
	"context"
	"errors"
	"io"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrPasswordChangesDisabled prevents local changes to provider-managed passwords.
	ErrPasswordChangesDisabled = errors.New("password changes unsupported in configured auth mode")
	// ErrInvalidProfile menandakan field profil tidak memenuhi aturan domain.
	ErrInvalidProfile = errors.New("profile input is invalid")
	// ErrInvalidCurrentPIN menandakan PIN lama tidak cocok.
	ErrInvalidCurrentPIN = errors.New("current PIN is invalid")
	// ErrInvalidCurrentPassword menandakan password lama tidak cocok.
	ErrInvalidCurrentPassword = errors.New("current password is invalid")
	// ErrInvalidPIN menandakan PIN baru bukan enam digit.
	ErrInvalidPIN = errors.New("new PIN is invalid")
	// ErrInvalidPassword menandakan password baru tidak memenuhi panjang aman.
	ErrInvalidPassword = errors.New("new password is invalid")
	// ErrCredentialReuse menandakan kredensial baru sama dengan kredensial lama.
	ErrCredentialReuse = errors.New("new credential reuses current value")
	// ErrAvatarStorageDisabled menandakan upload avatar belum dikonfigurasi.
	ErrAvatarStorageDisabled = errors.New("avatar storage is disabled")
	// ErrAvatarTooLarge menandakan avatar melebihi tiga MiB.
	ErrAvatarTooLarge = errors.New("avatar exceeds maximum size")
	// ErrInvalidAvatar menandakan file avatar bukan PNG/JPEG valid.
	ErrInvalidAvatar = errors.New("avatar is invalid")
	// ErrAvatarStorage menandakan operasi bucket avatar gagal.
	ErrAvatarStorage = errors.New("avatar storage failed")
)

// ProfileMutation adalah perubahan parsial profil akun sendiri.
type ProfileMutation struct {
	FirstName, LastName, PhoneNumber, Email string
	DateOfBirth, GenderID, PersonalAddress  string
	Fields                                  map[string]bool
	Avatar                                  *AvatarUpload
	RemoveAvatar                            bool
}

// AvatarUpload adalah file avatar dari multipart form.
type AvatarUpload struct {
	Reader                io.ReadSeeker
	Size                  int64
	ContentType, Filename string
}

// AvatarImage contains the only avatar reference persisted in PostgreSQL.
type AvatarImage struct{ URL string }

// AvatarStore saves avatars and deletes them through their persisted Supabase URL.
type AvatarStore interface {
	Save(context.Context, string, string, *AvatarUpload) (*AvatarImage, error)
	Delete(context.Context, string) error
}

// CredentialHasher mengisolasi bcrypt agar aturan perubahan kredensial dapat diuji deterministik.
type CredentialHasher interface {
	Hash(string) (string, error)
	Compare(string, string) error
}

type bcryptCredentialHasher struct{}

func (bcryptCredentialHasher) Hash(value string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(value), bcrypt.DefaultCost)
	return string(hash), err
}
func (bcryptCredentialHasher) Compare(hash, value string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(value))
}

type disabledAvatarStore struct{}

// NewDisabledAvatarStore membuat storage avatar yang menolak file tetapi membiarkan edit teks profil.
func NewDisabledAvatarStore() AvatarStore { return disabledAvatarStore{} }
func (disabledAvatarStore) Save(context.Context, string, string, *AvatarUpload) (*AvatarImage, error) {
	return nil, ErrAvatarStorageDisabled
}
func (disabledAvatarStore) Delete(context.Context, string) error { return ErrAvatarStorageDisabled }

// ServiceOption mengubah dependency opsional service user.
type ServiceOption func(*Service)

// WithCredentialHasher mengganti hasher kredensial, terutama untuk test.
func WithCredentialHasher(hasher CredentialHasher) ServiceOption {
	return func(service *Service) { service.hasher = hasher }
}

// WithAvatarStore memasang storage avatar yang digunakan edit profil.
func WithAvatarStore(store AvatarStore) ServiceOption {
	return func(service *Service) { service.avatars = store }
}

var sixDigitPIN = regexp.MustCompile(`^[0-9]{6}$`)

// UpdateProfile memperbarui field yang dikirim, mengelola avatar, dan mempertahankan field read-only.
func (service *Service) UpdateProfile(ctx context.Context, userID string, mutation ProfileMutation) (*User, error) {
	account, err := service.repository.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	// Work on a copy: rejected mutations must not change cached/fake repository state.
	copyAccount := *account
	account = &copyAccount
	isSet := func(field string) bool { return mutation.Fields == nil || mutation.Fields[field] }
	if isSet("first_name") {
		account.FirstName = strings.TrimSpace(mutation.FirstName)
	}
	if isSet("last_name") {
		account.LastName = strings.TrimSpace(mutation.LastName)
	}
	if isSet("phone_number") {
		account.PhoneNumber = strings.TrimSpace(mutation.PhoneNumber)
	}
	if isSet("email") {
		value := strings.TrimSpace(mutation.Email)
		if value == "" {
			account.Email = nil
		} else {
			parsed, parseErr := mail.ParseAddress(value)
			if parseErr != nil || parsed.Address != value || len(value) > 254 {
				return nil, ErrInvalidProfile
			}
			account.Email = &value
		}
	}
	if account.FirstName == "" || len([]rune(account.FirstName)) > 100 || len([]rune(account.LastName)) > 100 || len(account.PhoneNumber) < 8 || len(account.PhoneNumber) > 30 {
		return nil, ErrInvalidProfile
	}
	if err := service.applyPersonalDetails(ctx, account, mutation); err != nil {
		return nil, err
	}
	oldURL := account.AvatarURL
	var uploaded *AvatarImage
	if mutation.Avatar != nil {
		if account.OutletID == nil || *account.OutletID == "" {
			return nil, ErrInvalidProfile
		}
		uploaded, err = service.avatars.Save(ctx, *account.OutletID, account.ID, mutation.Avatar)
		if err != nil {
			return nil, err
		}
		account.AvatarURL = &uploaded.URL
	} else if isSet("remove_avatar") && mutation.RemoveAvatar {
		account.AvatarURL = nil
	}
	if service.settings == nil {
		if uploaded != nil {
			service.cleanupUnreferencedAvatar(ctx, userID, uploaded.URL)
		}
		return nil, errors.New("user settings repository unavailable")
	}
	if atomic, ok := service.repository.(atomicSettingsRepository); ok {
		account, err = atomic.PatchProfile(ctx, account, mutation, oldURL)
	} else {
		err = service.settings.UpdateProfile(ctx, account)
	}
	if err != nil {
		if uploaded != nil {
			service.cleanupUnreferencedAvatar(ctx, userID, uploaded.URL)
		}
		return nil, err
	}
	if oldURL != nil && (uploaded != nil || (isSet("remove_avatar") && mutation.RemoveAvatar)) {
		_ = service.avatars.Delete(ctx, *oldURL)
	}
	return account, nil
}

// ChangePIN memverifikasi PIN lama bila sudah ada lalu menyimpan bcrypt hash PIN baru.
func (service *Service) ChangePIN(ctx context.Context, userID, currentPIN, newPIN string) error {
	if !sixDigitPIN.MatchString(newPIN) {
		return ErrInvalidPIN
	}
	account, err := service.repository.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if account.PINHash != nil {
		if service.hasher.Compare(*account.PINHash, currentPIN) != nil {
			return ErrInvalidCurrentPIN
		}
		if service.hasher.Compare(*account.PINHash, newPIN) == nil {
			return ErrCredentialReuse
		}
	}
	hash, err := service.hasher.Hash(newPIN)
	if err != nil {
		return err
	}
	if service.settings == nil {
		return errors.New("user settings repository unavailable")
	}
	if atomic, ok := service.repository.(atomicSettingsRepository); ok {
		return atomic.ReplaceCredential(ctx, userID, "pin_hash", account.PINHash, hash)
	}
	return service.settings.UpdatePINHash(ctx, userID, hash)
}

// ChangePassword memverifikasi password lama dan menyimpan bcrypt hash password baru.
func (service *Service) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	if !service.passwordChangesAllowed {
		return ErrPasswordChangesDisabled
	}
	if len(currentPassword) < 8 || len(currentPassword) > 72 {
		return ErrInvalidCurrentPassword
	}
	if len(newPassword) < 8 || len(newPassword) > 72 {
		return ErrInvalidPassword
	}
	account, err := service.repository.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if service.hasher.Compare(account.PasswordHash, currentPassword) != nil {
		return ErrInvalidCurrentPassword
	}
	if service.hasher.Compare(account.PasswordHash, newPassword) == nil {
		return ErrCredentialReuse
	}
	hash, err := service.hasher.Hash(newPassword)
	if err != nil {
		return err
	}
	if service.settings == nil {
		return errors.New("user settings repository unavailable")
	}
	if atomic, ok := service.repository.(atomicSettingsRepository); ok {
		return atomic.ReplaceCredential(ctx, userID, "password_hash", &account.PasswordHash, hash)
	}
	return service.settings.UpdatePasswordHash(ctx, userID, hash)
}

// WithProfileClock injects the clock used for birth-date validation.
func WithProfileClock(now func() time.Time) ServiceOption { return func(s *Service) { s.now = now } }

// WithPasswordChangesAllowed configures whether the authentication provider permits local password changes.
func WithPasswordChangesAllowed(allowed bool) ServiceOption {
	return func(s *Service) { s.passwordChangesAllowed = allowed }
}

// cleanupUnreferencedAvatar avoids deleting a live photo after an ambiguous commit failure.
// If the database cannot confirm the reference, retain the potential orphan for reconciliation.
func (s *Service) cleanupUnreferencedAvatar(ctx context.Context, id, downloadURL string) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	current, e := s.repository.FindByID(cleanup, id)
	if e != nil && !errors.Is(e, ErrNotFound) {
		return
	}
	if current != nil && current.AvatarURL != nil && *current.AvatarURL == downloadURL {
		return
	}
	_ = s.avatars.Delete(cleanup, downloadURL)
}
