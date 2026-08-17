package registration

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newRegistrationRepositoryMock(t *testing.T) (*GORMRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	database, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB, PreferSimpleProtocol: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open GORM: %v", err)
	}
	return NewGORMRepository(database), mock
}

func TestGORMRepositoryCreatesRegistrationInOneTransaction(t *testing.T) {
	repository, mock := newRegistrationRepositoryMock(t)
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "roles" WHERE LOWER\(name\) = \$1`).WithArgs("admin", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("role-id", "admin"))
	mock.ExpectExec(`INSERT INTO "outlets"`).WithArgs(
		"Jl. Merdeka 10", "restaurant", sqlmock.AnyArg(), nil, sqlmock.AnyArg(), false, "Warung Dikasa", "081234567890", 0, sqlmock.AnyArg(),
	).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO "users"`).WithArgs(
		sqlmock.AnyArg(), now, "budi@example.com", "Budi", sqlmock.AnyArg(), false, "", sqlmock.AnyArg(), "hash", "081298765432", "role-id", sqlmock.AnyArg(), "budi",
	).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO "registration_otps"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	pending, err := repository.CreatePending(context.Background(), CreatePendingParams{
		BusinessName: "Warung Dikasa", BusinessAddress: "Jl. Merdeka 10", BusinessPhone: "081234567890",
		OwnerName: "Budi", OwnerDateOfBirth: now, OwnerPhone: "081298765432", OwnerEmail: "budi@example.com",
		Username: "budi", PasswordHash: "hash", VerificationEmail: "budi@example.com", CodeHash: "otp-hash",
		ExpiresAt: now.Add(5 * time.Minute), ResendAvailableAt: now.Add(time.Minute), MaxAttempts: 5,
	})

	if err != nil {
		t.Fatalf("create pending registration: %v", err)
	}
	if pending.UserID == "" || pending.OutletID == "" {
		t.Fatalf("expected linked IDs, got %+v", pending)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestGORMRepositoryFindsPendingByNormalizedEmail(t *testing.T) {
	repository, mock := newRegistrationRepositoryMock(t)
	query := regexp.QuoteMeta(`SELECT * FROM "registration_otps" WHERE LOWER(email) = LOWER($1) ORDER BY "registration_otps"."id" LIMIT $2`)
	mock.ExpectQuery(query).WithArgs("BUDI@example.com", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "code_hash"}).AddRow("otp-id", "budi@example.com", "hash"))

	pending, err := repository.FindPendingByEmail(context.Background(), " BUDI@example.com ")

	if err != nil || pending.ID != "otp-id" {
		t.Fatalf("find pending registration: %+v, %v", pending, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}
