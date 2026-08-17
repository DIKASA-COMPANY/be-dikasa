package registration

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	otpLifetime        = 5 * time.Minute
	resendCooldown     = time.Minute
	defaultMaxAttempts = 5
)

var (
	// ErrVerificationEmailInvalid menandakan email pilihan bukan email usaha atau pemilik.
	ErrVerificationEmailInvalid = errors.New("verification email is not one of the submitted emails")
	// ErrInvalidRegistration menandakan data registrasi tidak dapat diproses.
	ErrInvalidRegistration = errors.New("invalid registration")
	// ErrDeliveryFailed menandakan OTP gagal dikirim ke email tujuan.
	ErrDeliveryFailed = errors.New("OTP delivery failed")
	// ErrOTPInvalid menandakan kode OTP salah atau registrasi tidak ditemukan.
	ErrOTPInvalid = errors.New("invalid OTP")
	// ErrOTPExpired menandakan kode OTP melewati masa berlaku.
	ErrOTPExpired = errors.New("OTP expired")
	// ErrOTPAttemptsExceeded menandakan batas percobaan kode telah habis.
	ErrOTPAttemptsExceeded = errors.New("OTP attempts exceeded")
	// ErrResendTooSoon menandakan cooldown pengiriman ulang belum selesai.
	ErrResendTooSoon = errors.New("OTP resend cooldown active")
)

// Mailer mengirim kode OTP tanpa membocorkan detail transport ke use case.
type Mailer interface {
	// SendOTP mengirim satu kode ke email tujuan dengan informasi masa berlaku.
	SendOTP(ctx context.Context, to, code string, validFor time.Duration) error
}

// HMACHasher membuat keyed hash OTP agar kode pendek tidak dapat ditebak dari database saja.
type HMACHasher struct{ secret []byte }

// NewHMACHasher membuat hasher OTP menggunakan secret aplikasi.
func NewHMACHasher(secret string) *HMACHasher { return &HMACHasher{secret: []byte(secret)} }

// Hash menghasilkan HMAC-SHA256 terikat pada alamat email dan kode OTP.
func (hasher *HMACHasher) Hash(email, code string) string {
	mac := hmac.New(sha256.New, hasher.secret)
	_, _ = mac.Write([]byte(strings.ToLower(strings.TrimSpace(email)) + ":" + code))
	return hex.EncodeToString(mac.Sum(nil))
}

// Equal membandingkan hash OTP dengan waktu konstan.
func (hasher *HMACHasher) Equal(email, code, expected string) bool {
	actual := hasher.Hash(email, code)
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}

// Option mengubah dependency deterministik service, terutama untuk pengujian.
type Option func(*Service)

// WithClock mengganti sumber waktu service.
func WithClock(clock func() time.Time) Option { return func(service *Service) { service.now = clock } }

// WithCodeGenerator mengganti generator kode OTP service.
func WithCodeGenerator(generator func() (string, error)) Option {
	return func(service *Service) { service.generateCode = generator }
}

// WithBcryptCost mengganti biaya bcrypt, terutama agar unit test tetap cepat.
func WithBcryptCost(cost int) Option { return func(service *Service) { service.bcryptCost = cost } }

// Service menjalankan use case registrasi dan siklus hidup OTP.
type Service struct {
	repository   Repository
	mailer       Mailer
	hasher       *HMACHasher
	now          func() time.Time
	generateCode func() (string, error)
	bcryptCost   int
}

// NewService membuat service registrasi dengan kebijakan OTP lima menit dan cooldown satu menit.
func NewService(repository Repository, mailer Mailer, hasher *HMACHasher, options ...Option) *Service {
	service := &Service{repository: repository, mailer: mailer, hasher: hasher, now: time.Now, generateCode: generateOTP, bcryptCost: bcrypt.DefaultCost}
	for _, option := range options {
		option(service)
	}
	return service
}

// Register menyimpan akun nonaktif lalu mengirim OTP ke email yang dipilih.
func (service *Service) Register(ctx context.Context, registration Registration) (Result, error) {
	email, ok := selectedVerificationEmail(registration)
	if !ok {
		return Result{}, ErrVerificationEmailInvalid
	}
	now := service.now().UTC()
	birthDate, err := time.Parse("2006-01-02", registration.OwnerDateOfBirth)
	if err != nil || birthDate.After(now) {
		return Result{}, ErrInvalidRegistration
	}
	if len([]byte(registration.Password)) > 72 {
		return Result{}, ErrInvalidRegistration
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(registration.Password), service.bcryptCost)
	if err != nil {
		return Result{}, fmt.Errorf("hash registration password: %w", err)
	}
	code, err := service.generateCode()
	if err != nil {
		return Result{}, fmt.Errorf("generate registration OTP: %w", err)
	}
	pending, err := service.repository.CreatePending(ctx, CreatePendingParams{
		BusinessName: strings.TrimSpace(registration.BusinessName), BusinessAddress: strings.TrimSpace(registration.BusinessAddress),
		BusinessPhone: strings.TrimSpace(registration.BusinessPhone), BusinessEmail: normalizeOptionalEmail(registration.BusinessEmail),
		OwnerName: strings.TrimSpace(registration.OwnerName), OwnerDateOfBirth: birthDate, OwnerPhone: strings.TrimSpace(registration.OwnerPhone),
		OwnerEmail: strings.ToLower(strings.TrimSpace(registration.OwnerEmail)), Username: strings.TrimSpace(registration.Username),
		PasswordHash: string(passwordHash), VerificationEmail: email, CodeHash: service.hasher.Hash(email, code),
		ExpiresAt: now.Add(otpLifetime), ResendAvailableAt: now.Add(resendCooldown), MaxAttempts: defaultMaxAttempts,
	})
	if err != nil {
		return Result{}, err
	}
	if err := service.mailer.SendOTP(ctx, email, code, otpLifetime); err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrDeliveryFailed, err)
	}
	return resultFor(pending.Email, otpLifetime, resendCooldown), nil
}

// ConfirmOTP mengaktifkan user dan outlet secara atomik setelah kode berhasil diverifikasi.
func (service *Service) ConfirmOTP(ctx context.Context, email, code string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	pending, err := service.repository.FindPendingByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrOTPInvalid
		}
		return err
	}
	if pending.VerifiedAt != nil {
		return ErrAlreadyVerified
	}
	if pending.Attempts >= pending.MaxAttempts {
		return ErrOTPAttemptsExceeded
	}
	if !service.now().UTC().Before(pending.ExpiresAt) {
		return ErrOTPExpired
	}
	if !service.hasher.Equal(email, code, pending.CodeHash) {
		attempts, incrementErr := service.repository.IncrementFailedAttempts(ctx, pending.ID, pending.MaxAttempts)
		if incrementErr != nil {
			return incrementErr
		}
		if attempts >= pending.MaxAttempts {
			return ErrOTPAttemptsExceeded
		}
		return ErrOTPInvalid
	}
	return service.repository.MarkVerified(ctx, pending.ID, service.now().UTC())
}

// ResendOTP merotasi kode lama setelah cooldown dan mengirim kode baru.
func (service *Service) ResendOTP(ctx context.Context, email string) (Result, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	pending, err := service.repository.FindPendingByEmail(ctx, email)
	if err != nil {
		return Result{}, err
	}
	if pending.VerifiedAt != nil {
		return Result{}, ErrAlreadyVerified
	}
	now := service.now().UTC()
	if now.Before(pending.ResendAvailableAt) {
		return Result{}, ErrResendTooSoon
	}
	code, err := service.generateCode()
	if err != nil {
		return Result{}, fmt.Errorf("generate replacement OTP: %w", err)
	}
	params := ReplaceOTPParams{ID: pending.ID, CodeHash: service.hasher.Hash(email, code), ExpiresAt: now.Add(otpLifetime), ResendAvailableAt: now.Add(resendCooldown), UpdatedAt: now}
	if err := service.repository.ReplaceOTP(ctx, params); err != nil {
		return Result{}, err
	}
	if err := service.mailer.SendOTP(ctx, email, code, otpLifetime); err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrDeliveryFailed, err)
	}
	return resultFor(email, otpLifetime, resendCooldown), nil
}

// selectedVerificationEmail membatasi tujuan OTP pada dua email yang benar-benar dikirim saat registrasi.
func selectedVerificationEmail(registration Registration) (string, bool) {
	selected := strings.ToLower(strings.TrimSpace(registration.VerificationEmail))
	owner := strings.ToLower(strings.TrimSpace(registration.OwnerEmail))
	if selected == owner {
		return selected, true
	}
	if registration.BusinessEmail != nil && selected == strings.ToLower(strings.TrimSpace(*registration.BusinessEmail)) && selected != "" {
		return selected, true
	}
	return "", false
}

func normalizeOptionalEmail(email *string) *string {
	if email == nil || strings.TrimSpace(*email) == "" {
		return nil
	}
	normalized := strings.ToLower(strings.TrimSpace(*email))
	return &normalized
}

func resultFor(email string, lifetime, cooldown time.Duration) Result {
	return Result{Email: email, OTPExpiresIn: int(lifetime.Seconds()), ResendAvailableIn: int(cooldown.Seconds())}
}

// generateOTP menggunakan crypto/rand dan mempertahankan nol di depan untuk kode lima digit.
func generateOTP() (string, error) {
	number, err := rand.Int(rand.Reader, big.NewInt(100000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%05d", number.Int64()), nil
}
