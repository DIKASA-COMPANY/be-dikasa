package registration

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type stubRepository struct {
	created       CreatePendingParams
	pending       *Pending
	err           error
	failedAttempt int
	verifiedID    string
	replaced      ReplaceOTPParams
}

func (stub *stubRepository) CreatePending(_ context.Context, params CreatePendingParams) (*Pending, error) {
	stub.created = params
	if stub.err != nil {
		return nil, stub.err
	}
	if stub.pending != nil {
		return stub.pending, nil
	}
	return &Pending{ID: "registration-id", UserID: "user-id", OutletID: "outlet-id", Email: params.VerificationEmail, CodeHash: params.CodeHash, ExpiresAt: params.ExpiresAt, ResendAvailableAt: params.ResendAvailableAt}, nil
}

func (stub *stubRepository) FindPendingByEmail(context.Context, string) (*Pending, error) {
	return stub.pending, stub.err
}

func (stub *stubRepository) IncrementFailedAttempts(context.Context, string, int) (int, error) {
	stub.failedAttempt++
	return stub.failedAttempt, stub.err
}

func (stub *stubRepository) MarkVerified(_ context.Context, id string, _ time.Time) error {
	stub.verifiedID = id
	return stub.err
}

func (stub *stubRepository) ReplaceOTP(_ context.Context, params ReplaceOTPParams) error {
	stub.replaced = params
	return stub.err
}

type stubMailer struct {
	to   string
	code string
	err  error
}

func (stub *stubMailer) SendOTP(_ context.Context, to, code string, _ time.Duration) error {
	stub.to, stub.code = to, code
	return stub.err
}

func validRegistration() Registration {
	return Registration{
		BusinessName: "Warung Dikasa", BusinessAddress: "Jl. Merdeka 10", BusinessPhone: "081234567890",
		OwnerName: "Budi Santoso", OwnerDateOfBirth: "1990-01-02", OwnerPhone: "081298765432",
		OwnerEmail: "budi@example.com", Username: "budisantoso", Password: "rahasia123",
		VerificationEmail: "budi@example.com",
	}
}

func newTestService(repository Repository, mailer Mailer, now time.Time) *Service {
	return NewService(repository, mailer, NewHMACHasher("test-secret"),
		WithClock(func() time.Time { return now }),
		WithCodeGenerator(func() (string, error) { return "12345", nil }),
		WithBcryptCost(bcrypt.MinCost),
	)
}

func TestRegisterCreatesInactiveAccountAndSendsOTP(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	repository := &stubRepository{}
	mailer := &stubMailer{}
	service := newTestService(repository, mailer, now)
	input := validRegistration()
	businessEmail := "owner@warung.example"
	input.BusinessEmail = &businessEmail
	input.VerificationEmail = businessEmail

	result, err := service.Register(context.Background(), input)

	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if result.Email != businessEmail || mailer.to != businessEmail || mailer.code != "12345" {
		t.Fatalf("unexpected result or email: %+v, to=%q code=%q", result, mailer.to, mailer.code)
	}
	if repository.created.PasswordHash == input.Password || bcrypt.CompareHashAndPassword([]byte(repository.created.PasswordHash), []byte(input.Password)) != nil {
		t.Fatal("password must be bcrypt hashed")
	}
	if repository.created.CodeHash == "12345" || repository.created.CodeHash == "" {
		t.Fatal("OTP must be stored as a keyed hash")
	}
	if !repository.created.ExpiresAt.Equal(now.Add(5*time.Minute)) || !repository.created.ResendAvailableAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("unexpected OTP timing: %+v", repository.created)
	}
}

func TestRegisterRejectsVerificationEmailOutsideSubmittedEmails(t *testing.T) {
	service := newTestService(&stubRepository{}, &stubMailer{}, time.Now())
	input := validRegistration()
	input.VerificationEmail = "attacker@example.com"

	_, err := service.Register(context.Background(), input)

	if !errors.Is(err, ErrVerificationEmailInvalid) {
		t.Fatalf("expected ErrVerificationEmailInvalid, got %v", err)
	}
}

func TestRegisterRejectsPasswordBeyondBcryptLimit(t *testing.T) {
	service := newTestService(&stubRepository{}, &stubMailer{}, time.Now())
	input := validRegistration()
	input.Password = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	_, err := service.Register(context.Background(), input)

	if !errors.Is(err, ErrInvalidRegistration) {
		t.Fatalf("expected ErrInvalidRegistration, got %v", err)
	}
}

func TestRegisterRejectsFutureOwnerDateOfBirth(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	service := newTestService(&stubRepository{}, &stubMailer{}, now)
	input := validRegistration()
	input.OwnerDateOfBirth = "2026-08-18"

	_, err := service.Register(context.Background(), input)

	if !errors.Is(err, ErrInvalidRegistration) {
		t.Fatalf("expected ErrInvalidRegistration, got %v", err)
	}
}

func TestRegisterReportsDeliveryFailureWithoutExposingOTP(t *testing.T) {
	mailer := &stubMailer{err: errors.New("smtp unavailable")}
	service := newTestService(&stubRepository{}, mailer, time.Now())

	_, err := service.Register(context.Background(), validRegistration())

	if !errors.Is(err, ErrDeliveryFailed) {
		t.Fatalf("expected ErrDeliveryFailed, got %v", err)
	}
}

func TestConfirmOTPActivatesAccountOnce(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	hasher := NewHMACHasher("test-secret")
	repository := &stubRepository{pending: &Pending{
		ID: "registration-id", Email: "budi@example.com", CodeHash: hasher.Hash("budi@example.com", "12345"),
		ExpiresAt: now.Add(time.Minute), MaxAttempts: 5,
	}}
	service := newTestService(repository, &stubMailer{}, now)

	err := service.ConfirmOTP(context.Background(), "BUDI@example.com", "12345")

	if err != nil {
		t.Fatalf("confirm OTP: %v", err)
	}
	if repository.verifiedID != "registration-id" {
		t.Fatalf("expected registration to be verified, got %q", repository.verifiedID)
	}
}

func TestConfirmOTPRejectsExpiredAndWrongCodes(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	hasher := NewHMACHasher("test-secret")
	tests := []struct {
		name    string
		pending Pending
		code    string
		want    error
	}{
		{"expired", Pending{ExpiresAt: now.Add(-time.Second), MaxAttempts: 5}, "12345", ErrOTPExpired},
		{"wrong", Pending{CodeHash: hasher.Hash("budi@example.com", "12345"), ExpiresAt: now.Add(time.Minute), MaxAttempts: 5}, "54321", ErrOTPInvalid},
		{"attempts exhausted", Pending{CodeHash: "hash", ExpiresAt: now.Add(time.Minute), Attempts: 5, MaxAttempts: 5}, "12345", ErrOTPAttemptsExceeded},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.pending.ID = "registration-id"
			test.pending.Email = "budi@example.com"
			repository := &stubRepository{pending: &test.pending}
			service := newTestService(repository, &stubMailer{}, now)

			err := service.ConfirmOTP(context.Background(), "budi@example.com", test.code)

			if !errors.Is(err, test.want) {
				t.Fatalf("expected %v, got %v", test.want, err)
			}
			if errors.Is(test.want, ErrOTPInvalid) && repository.failedAttempt != 1 {
				t.Fatal("wrong OTP must increment failed attempts")
			}
		})
	}
}

func TestResendOTPRotatesCodeAfterCooldown(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	repository := &stubRepository{pending: &Pending{ID: "registration-id", Email: "budi@example.com", ResendAvailableAt: now.Add(-time.Second)}}
	mailer := &stubMailer{}
	service := newTestService(repository, mailer, now)

	result, err := service.ResendOTP(context.Background(), "budi@example.com")

	if err != nil {
		t.Fatalf("resend OTP: %v", err)
	}
	if repository.replaced.CodeHash == "" || mailer.code != "12345" || result.ResendAvailableIn != 60 {
		t.Fatalf("unexpected resend: %+v %+v", result, repository.replaced)
	}
}

func TestResendOTPEnforcesCooldown(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	service := newTestService(&stubRepository{pending: &Pending{ResendAvailableAt: now.Add(30 * time.Second)}}, &stubMailer{}, now)

	_, err := service.ResendOTP(context.Background(), "budi@example.com")

	if !errors.Is(err, ErrResendTooSoon) {
		t.Fatalf("expected ErrResendTooSoon, got %v", err)
	}
}
